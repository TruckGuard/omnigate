package main

import (
	"os"

	"github.com/omnigate/services/auth/src/models"
	"github.com/omnigate/services/auth/src/repository"
	"golang.org/x/crypto/bcrypt"
)

func seedData() {
	perms := []models.Permission{
		// users
		{ID: "read:users", Name: "Users: Read", Module: "auth"},
		{ID: "create:users", Name: "Users: Create", Module: "auth"},
		{ID: "update:users", Name: "Users: Update", Module: "auth"},
		{ID: "delete:users", Name: "Users: Delete", Module: "auth"},
		{ID: "change-role:users", Name: "Users: Change Role", Module: "auth"},
		{ID: "reset-password:users", Name: "Users: Reset Password", Module: "auth"},
		{ID: "manage:users", Name: "Users: Full Access", Module: "auth"},

		// roles
		{ID: "read:roles", Name: "Roles: Read", Module: "auth"},
		{ID: "create:roles", Name: "Roles: Create", Module: "auth"},
		{ID: "update:roles", Name: "Roles: Update", Module: "auth"},
		{ID: "delete:roles", Name: "Roles: Delete", Module: "auth"},
		{ID: "update-permissions:roles", Name: "Roles: Update Permissions", Module: "auth"},
		{ID: "manage:roles", Name: "Roles: Full Access", Module: "auth"},

		// api-keys
		{ID: "read:api-keys", Name: "API Keys: Read", Module: "auth"},
		{ID: "create:api-keys", Name: "API Keys: Create", Module: "auth"},
		{ID: "update:api-keys", Name: "API Keys: Update", Module: "auth"},
		{ID: "delete:api-keys", Name: "API Keys: Delete", Module: "auth"},
		{ID: "update-permissions:api-keys", Name: "API Keys: Update Permissions", Module: "auth"},
		{ID: "create-digest:api-keys", Name: "API Keys: Set Digest", Module: "auth"},
		{ID: "delete-digest:api-keys", Name: "API Keys: Clear Digest", Module: "auth"},
		{ID: "manage:api-keys", Name: "API Keys: Full Access", Module: "auth"},

		// audit
		{ID: "read:audit", Name: "Audit: View", Module: "auth"},

		// ingest (device-facing, not user roles)
		{ID: "ingest:events", Name: "Ingest: Create Event", Module: "ingestor"},
		{ID: "ingest:assume-source", Name: "Ingest: Assume Source Identity", Module: "ingestor"},

		// events
		{ID: "read:events", Name: "Events: Read", Module: "core"},
		{ID: "create:events", Name: "Events: Create", Module: "core"},
		{ID: "delete:events", Name: "Events: Delete", Module: "core"},
		{ID: "manage:events", Name: "Events: Full Access", Module: "core"},

		// transactions
		{ID: "read:transactions", Name: "Transactions: Read", Module: "core"},
		{ID: "create:transactions", Name: "Transactions: Create", Module: "core"},
		{ID: "update:transactions", Name: "Transactions: Update", Module: "core"},
		{ID: "delete:transactions", Name: "Transactions: Delete", Module: "core"},
		{ID: "close:transactions", Name: "Transactions: Close", Module: "core"},
		{ID: "manage:transactions", Name: "Transactions: Full Access", Module: "core"},

		// devices (endpoint path is /configs/devices)
		{ID: "read:devices", Name: "Devices: Read", Module: "core"},
		{ID: "create:devices", Name: "Devices: Create", Module: "core"},
		{ID: "update:devices", Name: "Devices: Update", Module: "core"},
		{ID: "delete:devices", Name: "Devices: Delete", Module: "core"},
		{ID: "trigger:devices", Name: "Devices: Trigger", Module: "core"},
		{ID: "manage:devices", Name: "Devices: Full Access", Module: "core"},

		// types
		{ID: "read:types", Name: "Event Types: Read", Module: "core"},
		{ID: "create:types", Name: "Event Types: Create", Module: "core"},
		{ID: "update:types", Name: "Event Types: Update", Module: "core"},
		{ID: "delete:types", Name: "Event Types: Delete", Module: "core"},
		{ID: "manage:types", Name: "Event Types: Full Access", Module: "core"},

		// gates
		{ID: "read:gates", Name: "Gates: Read", Module: "core"},
		{ID: "create:gates", Name: "Gates: Create", Module: "core"},
		{ID: "update:gates", Name: "Gates: Update", Module: "core"},
		{ID: "delete:gates", Name: "Gates: Delete", Module: "core"},
		{ID: "manage:gates", Name: "Gates: Full Access", Module: "core"},

		// profiles
		{ID: "read:profiles", Name: "Profiles: Read", Module: "core"},
		{ID: "create:profiles", Name: "Profiles: Create", Module: "core"},
		{ID: "update:profiles", Name: "Profiles: Update", Module: "core"},
		{ID: "delete:profiles", Name: "Profiles: Delete", Module: "core"},
		{ID: "manage:profiles", Name: "Profiles: Full Access", Module: "core"},
	}

	for _, p := range perms {
		repository.DB.Save(&p)
	}

	// --- Roles ---

	var adminRole models.Role
	repository.DB.FirstOrCreate(&adminRole, models.Role{Name: "admin", Description: "Full system access"})
	repository.DB.Model(&adminRole).Association("Permissions").Replace(perms)

	var managerRole models.Role
	repository.DB.FirstOrCreate(&managerRole, models.Role{Name: "manager", Description: "Manager (data editing)"})
	var managerPerms []models.Permission
	repository.DB.Where("id IN ?", []string{
		"read:users", "read:roles", "read:api-keys", "read:audit",
		"manage:events", "manage:transactions", "manage:devices",
		"manage:types", "manage:gates", "manage:profiles",
	}).Find(&managerPerms)
	repository.DB.Model(&managerRole).Association("Permissions").Replace(managerPerms)

	var operatorRole models.Role
	repository.DB.FirstOrCreate(&operatorRole, models.Role{Name: "operator", Description: "Operator (view and register)"})
	var operatorPerms []models.Permission
	repository.DB.Where("id IN ?", []string{
		"read:events", "create:events",
		"read:transactions", "create:transactions", "close:transactions",
		"read:devices", "read:types", "read:gates", "read:profiles",
	}).Find(&operatorPerms)
	repository.DB.Model(&operatorRole).Association("Permissions").Replace(operatorPerms)

	// --- Default admin user ---

	adminUsername := "admin"
	adminPassword := os.Getenv("ADMIN_DEFAULT_PASSWORD")
	var adminUser models.User
	if adminPassword == "" {
		println("WARN: ADMIN_DEFAULT_PASSWORD not set, skipping default admin creation")
	}
	err := repository.DB.Where("username = ?", adminUsername).First(&adminUser).Error
	if adminPassword != "" && err != nil {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), 12)
		newAdmin := models.User{
			Username:     adminUsername,
			PasswordHash: string(hashedPassword),
			RoleID:       adminRole.ID,
			Role:         adminRole,
		}
		if createErr := repository.DB.Create(&newAdmin).Error; createErr == nil {
			println("Default admin created: admin")
		}
	}

	// --- System API Keys ---

	workerKey := os.Getenv("WORKER_SYSTEM_KEY")
	if workerKey != "" {
		h := repository.HashKey(workerKey)
		var existingKey models.APIKey
		err := repository.DB.Where("key_hash = ?", h).First(&existingKey).Error
		if err != nil {
			var workerPerms []models.Permission
			repository.DB.Where("id IN ?", []string{
				"ingest:events",
				"manage:events", "manage:transactions",
				"read:devices", "read:types",
			}).Find(&workerPerms)
			repository.DB.Create(&models.APIKey{
				KeyHash:     h,
				OwnerName:   "System Worker",
				IsActive:    true,
				GateID:      "system",
				Permissions: workerPerms,
			})
			println("API key for system worker created")
		}
	}

	pullerKey := os.Getenv("PULLER_API_KEY")
	if pullerKey != "" {
		h := repository.HashKey(pullerKey)
		var existingKey models.APIKey
		err := repository.DB.Where("key_hash = ?", h).First(&existingKey).Error
		if err != nil {
			var pullerPerms []models.Permission
			repository.DB.Where("id IN ?", []string{
				"ingest:events", "ingest:assume-source",
			}).Find(&pullerPerms)
			repository.DB.Create(&models.APIKey{
				KeyHash:     h,
				OwnerName:   "Puller",
				IsActive:    true,
				GateID:      "system",
				Permissions: pullerPerms,
			})
			println("API key for puller created")
		}
	}

	// --- Policy Rules ---
	// Each rule maps one method + path pattern to exactly one required permission.
	// Custom actions (close, trigger, change-role, etc.) have their own explicit rules
	// instead of relying on CRUD inference.

	rules := []models.PolicyRule{
		// Auth: Users
		{Method: "GET", PathPattern: `^/api/auth/admin/users.*`, RequiredPermission: "read:users", Description: "List/view users"},
		{Method: "POST", PathPattern: `^/api/auth/register$`, RequiredPermission: "create:users", Description: "Register user"},
		{Method: "DELETE", PathPattern: `^/api/auth/admin/users/[^/]+$`, RequiredPermission: "delete:users", Description: "Delete user"},
		{Method: "PUT", PathPattern: `^/api/auth/admin/users/[^/]+/role$`, RequiredPermission: "change-role:users", Description: "Change user role"},
		{Method: "POST", PathPattern: `^/api/auth/admin/users/[^/]+/reset-password$`, RequiredPermission: "reset-password:users", Description: "Reset user password"},

		// Auth: Roles
		{Method: "GET", PathPattern: `^/api/auth/admin/roles.*`, RequiredPermission: "read:roles", Description: "List/view roles"},
		{Method: "GET", PathPattern: `^/api/auth/admin/permissions$`, RequiredPermission: "read:roles,read:api-keys", Description: "List all permissions"},
		{Method: "POST", PathPattern: `^/api/auth/admin/roles$`, RequiredPermission: "create:roles", Description: "Create role"},
		{Method: "PUT", PathPattern: `^/api/auth/admin/roles/[^/]+$`, RequiredPermission: "update:roles", Description: "Update role"},
		{Method: "DELETE", PathPattern: `^/api/auth/admin/roles/[^/]+$`, RequiredPermission: "delete:roles", Description: "Delete role"},
		{Method: "POST", PathPattern: `^/api/auth/admin/roles/[^/]+/permissions$`, RequiredPermission: "update-permissions:roles", Description: "Assign role permissions"},

		// Auth: API Keys
		{Method: "GET", PathPattern: `^/api/auth/admin/keys.*`, RequiredPermission: "read:api-keys", Description: "List/view API keys"},
		{Method: "POST", PathPattern: `^/api/auth/admin/keys$`, RequiredPermission: "create:api-keys", Description: "Create API key"},
		{Method: "PUT", PathPattern: `^/api/auth/admin/keys/[^/]+$`, RequiredPermission: "update:api-keys", Description: "Update API key"},
		{Method: "DELETE", PathPattern: `^/api/auth/admin/keys/[^/]+$`, RequiredPermission: "delete:api-keys", Description: "Delete API key"},
		{Method: "PUT", PathPattern: `^/api/auth/admin/keys/[^/]+/permissions$`, RequiredPermission: "update-permissions:api-keys", Description: "Update key permissions"},
		{Method: "POST", PathPattern: `^/api/auth/admin/keys/[^/]+/digest$`, RequiredPermission: "create-digest:api-keys", Description: "Set key digest credentials"},
		{Method: "DELETE", PathPattern: `^/api/auth/admin/keys/[^/]+/digest$`, RequiredPermission: "delete-digest:api-keys", Description: "Clear key digest credentials"},

		// Auth: Audit
		{Method: "GET", PathPattern: `^/api/auth/audit.*`, RequiredPermission: "read:audit", Description: "Audit log"},

		// Ingestor
		{Method: "POST", PathPattern: `^/ingest/.*`, RequiredPermission: "ingest:events", Description: "Ingest event"},

		// Core: Events
		{Method: "GET", PathPattern: `^/api/v1/events.*`, RequiredPermission: "read:events", Description: "List/view events"},
		{Method: "POST", PathPattern: `^/api/v1/events$`, RequiredPermission: "create:events", Description: "Create event"},
		{Method: "DELETE", PathPattern: `^/api/v1/events/[^/]+$`, RequiredPermission: "delete:events", Description: "Delete event"},

		// Core: Transactions
		{Method: "GET", PathPattern: `^/api/v1/transactions.*`, RequiredPermission: "read:transactions", Description: "List/view transactions"},
		{Method: "POST", PathPattern: `^/api/v1/transactions$`, RequiredPermission: "create:transactions", Description: "Create transaction"},
		{Method: "PUT", PathPattern: `^/api/v1/transactions/[^/]+$`, RequiredPermission: "update:transactions", Description: "Update transaction"},
		{Method: "DELETE", PathPattern: `^/api/v1/transactions/[^/]+$`, RequiredPermission: "delete:transactions", Description: "Delete transaction"},
		{Method: "POST", PathPattern: `^/api/v1/transactions/[^/]+/close$`, RequiredPermission: "close:transactions", Description: "Close transaction"},

		// Core: Devices (/configs/devices/... in URL, "devices" as permission resource)
		{Method: "GET", PathPattern: `^/api/v1/configs/devices.*`, RequiredPermission: "read:devices", Description: "List/view device configs"},
		{Method: "POST", PathPattern: `^/api/v1/configs/devices$`, RequiredPermission: "create:devices", Description: "Create device config"},
		{Method: "PUT", PathPattern: `^/api/v1/configs/devices/[^/]+$`, RequiredPermission: "update:devices", Description: "Update device config"},
		{Method: "DELETE", PathPattern: `^/api/v1/configs/devices/[^/]+$`, RequiredPermission: "delete:devices", Description: "Delete device config"},
		{Method: "POST", PathPattern: `^/api/v1/configs/devices/[^/]+/trigger$`, RequiredPermission: "trigger:devices", Description: "Trigger device"},

		// Core: Event Types
		{Method: "GET", PathPattern: `^/api/v1/types.*`, RequiredPermission: "read:types", Description: "List/view event types"},
		{Method: "POST", PathPattern: `^/api/v1/types$`, RequiredPermission: "create:types", Description: "Create event type"},
		{Method: "PUT", PathPattern: `^/api/v1/types/[^/]+$`, RequiredPermission: "update:types", Description: "Update event type"},
		{Method: "DELETE", PathPattern: `^/api/v1/types/[^/]+$`, RequiredPermission: "delete:types", Description: "Delete event type"},

		// Core: Gates
		{Method: "GET", PathPattern: `^/api/v1/gates.*`, RequiredPermission: "read:gates", Description: "List/view gates"},
		{Method: "POST", PathPattern: `^/api/v1/gates$`, RequiredPermission: "create:gates", Description: "Create gate"},
		{Method: "PUT", PathPattern: `^/api/v1/gates/[^/]+`, RequiredPermission: "update:gates", Description: "Update gate or gate settings"},
		{Method: "DELETE", PathPattern: `^/api/v1/gates/[^/]+$`, RequiredPermission: "delete:gates", Description: "Delete gate"},

		// Core: Profiles (/me is intentionally absent — unmanaged, accessible to any authenticated user)
		{Method: "GET", PathPattern: `^/api/v1/profiles$`, RequiredPermission: "read:profiles", Description: "List profiles"},
		{Method: "GET", PathPattern: `^/api/v1/profiles/[0-9a-fA-F]`, RequiredPermission: "read:profiles", Description: "View profile by UUID"},
		{Method: "POST", PathPattern: `^/api/v1/profiles$`, RequiredPermission: "create:profiles", Description: "Create profile"},
		{Method: "PUT", PathPattern: `^/api/v1/profiles/[0-9a-fA-F]`, RequiredPermission: "update:profiles", Description: "Update profile by UUID"},
		{Method: "DELETE", PathPattern: `^/api/v1/profiles/[0-9a-fA-F]`, RequiredPermission: "delete:profiles", Description: "Delete profile by UUID"},
	}

	for _, r := range rules {
		var existing models.PolicyRule
		if err := repository.DB.Where("method = ? AND path_pattern = ?", r.Method, r.PathPattern).First(&existing).Error; err != nil {
			repository.DB.Create(&r)
		} else {
			existing.RequiredPermission = r.RequiredPermission
			existing.Description = r.Description
			repository.DB.Save(&existing)
		}
	}

	// --- Permission Hierarchy ---
	// manage:X covers all child permissions of that resource.
	// No cross-resource links — those caused unintended permission leaks.

	hierarchy := []models.PermissionHierarchy{
		// manage:users
		{ParentID: "manage:users", ChildID: "read:users"},
		{ParentID: "manage:users", ChildID: "create:users"},
		{ParentID: "manage:users", ChildID: "update:users"},
		{ParentID: "manage:users", ChildID: "delete:users"},
		{ParentID: "manage:users", ChildID: "change-role:users"},
		{ParentID: "manage:users", ChildID: "reset-password:users"},
		{ParentID: "manage:users", ChildID: "read:roles"},
		{ParentID: "manage:users", ChildID: "read:profiles"},

		// manage:roles
		{ParentID: "manage:roles", ChildID: "read:roles"},
		{ParentID: "manage:roles", ChildID: "create:roles"},
		{ParentID: "manage:roles", ChildID: "update:roles"},
		{ParentID: "manage:roles", ChildID: "delete:roles"},
		{ParentID: "manage:roles", ChildID: "update-permissions:roles"},

		// manage:api-keys
		{ParentID: "manage:api-keys", ChildID: "read:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "create:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "update:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "delete:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "update-permissions:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "create-digest:api-keys"},
		{ParentID: "manage:api-keys", ChildID: "delete-digest:api-keys"},

		// manage:events
		{ParentID: "manage:events", ChildID: "read:events"},
		{ParentID: "manage:events", ChildID: "create:events"},
		{ParentID: "manage:events", ChildID: "delete:events"},

		// manage:transactions
		{ParentID: "manage:transactions", ChildID: "read:transactions"},
		{ParentID: "manage:transactions", ChildID: "create:transactions"},
		{ParentID: "manage:transactions", ChildID: "update:transactions"},
		{ParentID: "manage:transactions", ChildID: "delete:transactions"},
		{ParentID: "manage:transactions", ChildID: "close:transactions"},

		// manage:devices
		{ParentID: "manage:devices", ChildID: "read:devices"},
		{ParentID: "manage:devices", ChildID: "create:devices"},
		{ParentID: "manage:devices", ChildID: "update:devices"},
		{ParentID: "manage:devices", ChildID: "delete:devices"},
		{ParentID: "manage:devices", ChildID: "trigger:devices"},

		// manage:types
		{ParentID: "manage:types", ChildID: "read:types"},
		{ParentID: "manage:types", ChildID: "create:types"},
		{ParentID: "manage:types", ChildID: "update:types"},
		{ParentID: "manage:types", ChildID: "delete:types"},

		// manage:gates
		{ParentID: "manage:gates", ChildID: "read:gates"},
		{ParentID: "manage:gates", ChildID: "create:gates"},
		{ParentID: "manage:gates", ChildID: "update:gates"},
		{ParentID: "manage:gates", ChildID: "delete:gates"},

		// manage:profiles
		{ParentID: "manage:profiles", ChildID: "read:profiles"},
		{ParentID: "manage:profiles", ChildID: "create:profiles"},
		{ParentID: "manage:profiles", ChildID: "update:profiles"},
		{ParentID: "manage:profiles", ChildID: "delete:profiles"},

		// read:users implies read:roles (need roles to assign) and read:profiles
		{ParentID: "read:users", ChildID: "read:roles"},
		{ParentID: "read:users", ChildID: "read:profiles"},
	}

	for _, h := range hierarchy {
		repository.DB.Where("parent_id = ? AND child_id = ?", h.ParentID, h.ChildID).FirstOrCreate(&h)
	}
}
