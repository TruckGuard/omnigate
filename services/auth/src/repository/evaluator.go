package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/omnigate/services/auth/src/models"
)
var PermissionHierarchy = make(map[string][]string)

// LoadPermissionHierarchy loads the permission hierarchy from the database.
func LoadPermissionHierarchy() {
	var relations []models.PermissionHierarchy
	if err := DB.Find(&relations).Error; err != nil {
		slog.Error("Failed to load permission hierarchy from DB", "error", err)
		return
	}

	newHierarchy := make(map[string][]string)
	for _, r := range relations {
		newHierarchy[r.ParentID] = append(newHierarchy[r.ParentID], r.ChildID)
	}

	PermissionHierarchy = newHierarchy
	slog.Info("Permission hierarchy loaded from DB", "parents", len(newHierarchy))
}

// CheckAccess checks access using regex-based policy rules and caches results in Redis.
func CheckAccess(method, path string) ([]string, bool, error) {
	cacheKey := fmt.Sprintf("auth:policy:%s:%s", strings.ToUpper(method), path)

	// 1. Try to get from Redis
	if val, _ := RDB.Get(context.Background(), cacheKey).Result(); val != "" {
		if val == "UNMANAGED" {
			slog.Debug("Policy cache hit: unmanaged path", "path", path)
			return nil, false, nil // Not managed by policies
		}
		var reqPerms []string
		json.Unmarshal([]byte(val), &reqPerms)
		slog.Debug("Policy cache hit: rules found", "path", path, "required_perms", reqPerms)
		return reqPerms, true, nil // Managed, return list of required permissions
	}

	slog.Info("Policy cache miss: querying database", "method", method, "path", path)

	// 2. If not in cache — query Postgres
	var allRulesForPath []models.PolicyRule
	if err := DB.Order("id asc").Where("? ~ path_pattern", path).Find(&allRulesForPath).Error; err != nil {
		return nil, false, err
	}

	if len(allRulesForPath) == 0 {
		// Path not managed by any rules
		RDB.Set(context.Background(), cacheKey, "UNMANAGED", 15*time.Minute)
		return nil, false, nil
	}

	// 3. Filter rules by method
	var requiredPerms []string
	for _, rule := range allRulesForPath {
		if rule.Method == "*" || strings.EqualFold(rule.Method, method) {
			for _, p := range strings.Split(rule.RequiredPermission, ",") {
				requiredPerms = append(requiredPerms, strings.TrimSpace(p))
			}
		} else if strings.ToUpper(rule.Method) == "CRUD" {
			action := "read"
			switch strings.ToUpper(method) {
			case "POST":
				action = "create"
			case "PUT", "PATCH":
				action = "update"
			case "DELETE":
				action = "delete"
			case "GET":
				action = "read"
			}
			for _, res := range strings.Split(rule.RequiredPermission, ",") {
				requiredPerms = append(requiredPerms, action+":"+strings.TrimSpace(res))
			}
		}
	}

	// 4. Cache the result (even if the list is empty — means managed path but method not allowed)
	val, _ := json.Marshal(requiredPerms)
	RDB.Set(context.Background(), cacheKey, string(val), 15*time.Minute)

	return requiredPerms, true, nil
}

// EvaluateAccess combines policy checking and user permission evaluation.
func EvaluateAccess(method, path string, userPerms []string) (bool, error) {
	reqPerms, isManaged, err := CheckAccess(method, path)
	if err != nil {
		return false, err
	}

	if !isManaged {
		return true, nil // Allow by default for unmanaged paths
	}

	// If path is managed but no rule matched the method
	if len(reqPerms) == 0 {
		slog.Warn("Access denied: path is managed but method not allowed", "method", method, "path", path)
		return false, nil
	}
	// Check if user has at least one of the required permissions
	for _, rp := range reqPerms {
		if rp != "" && HasPermission(userPerms, rp) {
			return true, nil
		}
	}

	slog.Warn("Access denied: insufficient permissions", "method", method, "path", path, "required", reqPerms)
	return false, nil
}

// ValidatePermissions checks if all targetPerms are covered by userPerms (respecting hierarchy).
// This prevents permission escalation when assigning roles or keys.
func ValidatePermissions(userPerms []string, targetPerms []string) error {
	for _, tp := range targetPerms {
		if !HasPermission(userPerms, tp) {
			return fmt.Errorf("insufficient permission to grant: %s", tp)
		}
	}
	return nil
}

func HasPermission(userPerms []string, required string) bool {
	if required == "" {
		return false
	}

	for _, p := range userPerms {
		if p == "admin" || p == required {
			return true
		}

		// Check via hierarchy (recursively)
		if deps, ok := PermissionHierarchy[p]; ok {
			for _, dep := range deps {
				if HasPermission([]string{dep}, required) {
					return true
				}
			}
		}

		// Format: action:resource[:scope] (e.g., read:cameras:all or update:events)
		partsUser := strings.Split(p, ":")
		partsReq := strings.Split(required, ":")

		// Minimum: action:resource
		if len(partsUser) < 2 || len(partsReq) < 2 {
			continue
		}

		actionUser := partsUser[0]
		resourceUser := partsUser[1]
		scopeUser := ""
		if len(partsUser) > 2 {
			scopeUser = partsUser[2]
		}

		actionReq := partsReq[0]
		resourceReq := partsReq[1]
		scopeReq := ""
		if len(partsReq) > 2 {
			scopeReq = partsReq[2]
		}

		// 1. Check resource (support wildcard)
		if resourceUser != "*" && resourceUser != resourceReq {
			continue
		}

		// 2. Check scope
		// If user has 'all', they can do everything within the resource.
		// If scopes match - ok.
		// If user doesn't have 'all' and a specific scope is requested - deny (unless they are the same).
		scopeMatch := false
		if scopeUser == "all" {
			scopeMatch = true
		} else if scopeUser == scopeReq {
			scopeMatch = true
		}

		if !scopeMatch {
			continue
		}

		// 3. Action hierarchy:
		// manage > delete > update/validate > create > read
		if actionUser == "manage" || actionUser == "admin" {
			return true
		}

		switch actionReq {
		case "read":
			// Any action (create/update/delete/validate) allows read
			return true
		case "create":
			if actionUser == "create" || actionUser == "update" || actionUser == "delete" || actionUser == "manage" {
				return true
			}
		case "update", "validate":
			if actionUser == "update" || actionUser == "validate" || actionUser == "delete" || actionUser == "manage" {
				return true
			}
		case "delete":
			if actionUser == "delete" || actionUser == "manage" {
				return true
			}
		}
	}
	return false
}

// ExpandPermissions expands the user's permission list, adding all dependent permissions according to the hierarchy.
func ExpandPermissions(userPerms []string) []string {
	result := make(map[string]bool)
	var queue []string

	for _, p := range userPerms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !result[p] {
			result[p] = true
			queue = append(queue, p)
		}
	}

	for i := 0; i < len(queue); i++ {
		p := queue[i]

		// Add base permission for permissions with :all suffix (e.g., manage:users:all -> manage:users)
		if strings.HasSuffix(p, ":all") {
			basePerm := strings.TrimSuffix(p, ":all")
			if !result[basePerm] {
				result[basePerm] = true
				queue = append(queue, basePerm)
			}

			if deps, ok := PermissionHierarchy[basePerm]; ok {
				for _, dep := range deps {
					depAll := dep + ":all"
					if !result[depAll] {
						result[depAll] = true
						queue = append(queue, depAll)
					}
				}
			}
		}

		if deps, ok := PermissionHierarchy[p]; ok {
			for _, dep := range deps {
				if !result[dep] {
					result[dep] = true
					queue = append(queue, dep)
				}
			}
		}
	}

	final := make([]string, 0, len(result))
	for p := range result {
		final = append(final, p)
	}
	return final
}
