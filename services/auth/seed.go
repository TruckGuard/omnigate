package main

import (
	"os"

	"github.com/omnigate/services/auth/src/models"
	"github.com/omnigate/services/auth/src/repository"
	"golang.org/x/crypto/bcrypt"
)

func genCRUDPerms(resource, name, module string, includeAll bool) []models.Permission {
	perms := []models.Permission{
		{ID: "read:" + resource, Name: name + ": Read", Module: module},
		{ID: "create:" + resource, Name: name + ": Create", Module: module},
		{ID: "update:" + resource, Name: name + ": Update", Module: module},
		{ID: "delete:" + resource, Name: name + ": Delete", Module: module},
		{ID: "manage:" + resource, Name: name + ": Full Access", Module: module},
	}
	if includeAll {
		perms = append(perms, []models.Permission{
			{ID: "read:" + resource + ":all", Name: name + ": Read (all)", Module: module},
			{ID: "manage:" + resource + ":all", Name: name + ": Full Access (all)", Module: module},
		}...)
	}
	return perms
}

func seedData() {
	var perms []models.Permission

	// Auth
	perms = append(perms, genCRUDPerms("users", "Users", "auth", true)...)
	perms = append(perms, genCRUDPerms("roles", "Roles", "auth", false)...)
	perms = append(perms, genCRUDPerms("keys", "API Keys", "auth", false)...)
	perms = append(perms, models.Permission{ID: "read:audit", Name: "Audit: View", Module: "auth"})

	// Ingest
	perms = append(perms, models.Permission{ID: "ingest:events", Name: "Ingest: Create", Module: "ingestor"})
	perms = append(perms, models.Permission{ID: "ingest:assume-source", Name: "Ingest: Assume Source Identity", Module: "ingestor"})

	// Core
	perms = append(perms, genCRUDPerms("events", "Events", "core", true)...)
	perms = append(perms, genCRUDPerms("transactions", "Transactions", "core", true)...)
	perms = append(perms, genCRUDPerms("configs", "Device Configs", "core", true)...)
	perms = append(perms, genCRUDPerms("types", "Event Types", "core", true)...)
	perms = append(perms, genCRUDPerms("gates", "Gates", "core", true)...)
	perms = append(perms, genCRUDPerms("profiles", "User Profiles", "core", true)...)

	for _, p := range perms {
		repository.DB.Save(&p)
	}

	// 1. Admin - has everything
	var adminRole models.Role
	repository.DB.FirstOrCreate(&adminRole, models.Role{Name: "admin", Description: "Full system access"})
	repository.DB.Model(&adminRole).Association("Permissions").Replace(perms)

	// 2. Manager - can edit but not delete generally, has good access
	var managerRole models.Role
	repository.DB.FirstOrCreate(&managerRole, models.Role{Name: "manager", Description: "Manager (data editing)"})
	managerPermIDs := []string{
		"manage:users", "manage:events", "manage:transactions", "manage:configs", "manage:types",
		"manage:gates", "manage:profiles",
		"read:roles", "read:keys", "read:audit",
	}
	var managerPerms []models.Permission
	repository.DB.Where("id IN ?", managerPermIDs).Find(&managerPerms)
	repository.DB.Model(&managerRole).Association("Permissions").Replace(managerPerms)

	// 3. Operator - view and create events/transactions only
	var operatorRole models.Role
	repository.DB.FirstOrCreate(&operatorRole, models.Role{Name: "operator", Description: "Operator (view and register)"})
	operatorPermIDs := []string{
		"read:events", "create:events", "manage:events",
		"read:transactions", "create:transactions", "manage:transactions",
		"read:configs", "read:types", "read:gates",
	}
	var operatorPerms []models.Permission
	repository.DB.Where("id IN ?", operatorPermIDs).Find(&operatorPerms)
	repository.DB.Model(&operatorRole).Association("Permissions").Replace(operatorPerms)

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

	workerKey := os.Getenv("WORKER_SYSTEM_KEY")
	if workerKey != "" {
		h := repository.HashKey(workerKey)
		var existingKey models.APIKey
		err := repository.DB.Where("key_hash = ?", h).First(&existingKey).Error
		if err != nil {
			var workerPerms []models.Permission
			repository.DB.Where("id IN ?", []string{"manage:events", "manage:transactions", "read:configs:all", "read:types:all", "ingest:events"}).Find(&workerPerms)

			newKey := models.APIKey{
				KeyHash:     h,
				OwnerName:   "System Worker",
				IsActive:    true,
				GateID:      "system",
				Permissions: workerPerms,
			}
			repository.DB.Create(&newKey)
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
			repository.DB.Where("id IN ?", []string{"ingest:events", "ingest:assume-source"}).Find(&pullerPerms)

			newKey := models.APIKey{
				KeyHash:     h,
				OwnerName:   "Puller",
				IsActive:    true,
				GateID:      "system",
				Permissions: pullerPerms,
			}
			repository.DB.Create(&newKey)
			println("API key for puller created")
		}
	}


	rules := []models.PolicyRule{
		// Auth Service
		{Method: "POST", PathPattern: `^/api/auth/register$`, RequiredPermission: "manage:users", Description: "User registration"},
		{Method: "GET", PathPattern: `^/api/auth/admin/users.*`, RequiredPermission: "read:users", Description: "View users"},
		{Method: "PUT", PathPattern: `^/api/auth/admin/users/.*/role$`, RequiredPermission: "manage:users", Description: "Change user role"},
		{Method: "DELETE", PathPattern: `^/api/auth/admin/users/.*`, RequiredPermission: "manage:users", Description: "Delete user"},
		{Method: "POST", PathPattern: `^/api/auth/admin/users/.*/reset-password$`, RequiredPermission: "manage:users", Description: "Reset user password"},
		{Method: "GET", PathPattern: `^/api/auth/admin/roles.*`, RequiredPermission: "read:roles", Description: "View roles"},
		{Method: "POST", PathPattern: `^/api/auth/admin/roles.*`, RequiredPermission: "manage:roles", Description: "Create roles"},
		{Method: "CRUD", PathPattern: `^/api/auth/admin/roles/.*`, RequiredPermission: "roles", Description: "Manage roles"},
		{Method: "GET", PathPattern: `^/api/auth/admin/keys.*`, RequiredPermission: "read:keys", Description: "View keys"},
		{Method: "POST", PathPattern: `^/api/auth/admin/keys.*`, RequiredPermission: "manage:keys", Description: "Create keys"},
		{Method: "CRUD", PathPattern: `^/api/auth/admin/keys/.*`, RequiredPermission: "keys", Description: "Manage keys"},
		{Method: "GET", PathPattern: `^/api/auth/admin/permissions$`, RequiredPermission: "read:roles", Description: "List all permissions"},

		// Ingestor Service
		{Method: "POST", PathPattern: `^/ingest/.*`, RequiredPermission: "ingest:events", Description: "Ingest data"},

		// Core Service: CONFIGS
		{Method: "CRUD", PathPattern: `^/api/v1/configs/device.*`, RequiredPermission: "configs,configs:all", Description: "Manage device configs"},

		// Core Service: EVENTS
		{Method: "CRUD", PathPattern: `^/api/v1/events.*`, RequiredPermission: "events,events:all", Description: "Manage events"},

		// Core Service: TRANSACTIONS
		{Method: "CRUD", PathPattern: `^/api/v1/transactions.*`, RequiredPermission: "transactions,transactions:all", Description: "Manage transactions"},

		// Core Service: TYPES
		{Method: "CRUD", PathPattern: `^/api/v1/types.*`, RequiredPermission: "types,types:all", Description: "Manage event types"},

		// Core Service: GATES
		{Method: "CRUD", PathPattern: `^/api/v1/gates.*`, RequiredPermission: "gates,gates:all", Description: "Manage gates"},

		// Core Service: PROFILES
		{Method: "CRUD", PathPattern: `^/api/v1/profiles.*`, RequiredPermission: "profiles,profiles:all", Description: "Manage user profiles"},

		// Audit
		{Method: "GET", PathPattern: `^/api/auth/audit.*`, RequiredPermission: "read:audit", Description: "Audit log"},
	}

	for _, r := range rules {
		var existing models.PolicyRule
		if err := repository.DB.Where("method = ? AND path_pattern = ?", r.Method, r.PathPattern).First(&existing).Error; err != nil {
			repository.DB.Create(&r)
		} else {
			existing.RequiredPermission = r.RequiredPermission
			repository.DB.Save(&existing)
		}
	}

	// Автоматична генерація ієрархії для CRUD-сутностей
	var hierarchy []models.PermissionHierarchy
	
	// Список усіх ресурсів, які підтримують CRUD (згідно з функцією genCRUDPerms у цьому ж файлі)
	crudResources := []string{
		"users", "roles", "keys", 
		"events", "transactions", "configs", 
		"types", "gates", "profiles",
	}

	for _, res := range crudResources {
		managePerm := "manage:" + res
		hierarchy = append(hierarchy,
			models.PermissionHierarchy{ParentID: managePerm, ChildID: "read:" + res},
			models.PermissionHierarchy{ParentID: managePerm, ChildID: "create:" + res},
			models.PermissionHierarchy{ParentID: managePerm, ChildID: "update:" + res},
			models.PermissionHierarchy{ParentID: managePerm, ChildID: "delete:" + res},
		)
	}

	// Крос-ресурсні зв'язки:

	// Управління/перегляд користувачів → перегляд ролей (щоб можна було призначати роль)
	hierarchy = append(hierarchy, models.PermissionHierarchy{ParentID: "manage:users", ChildID: "read:roles"})

	// Перегляд/управління пристроями → перегляд API-ключів (для відображення назви пристрою)
	hierarchy = append(hierarchy, models.PermissionHierarchy{ParentID: "read:configs", ChildID: "read:keys"})
	hierarchy = append(hierarchy, models.PermissionHierarchy{ParentID: "manage:configs", ChildID: "manage:keys"})

	// Збереження ієрархії в БД
	for _, h := range hierarchy {
		repository.DB.Where("parent_id = ? AND child_id = ?", h.ParentID, h.ChildID).FirstOrCreate(&h)
	}
}
