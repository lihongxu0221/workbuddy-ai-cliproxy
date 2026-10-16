package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store 认证凭据仓库接口与管理器
type Store struct {
	authDir  string
	accounts map[string]*WorkBuddyAuth
	mu       sync.RWMutex
}

var (
	defaultStore *Store
	onceStore    sync.Once
)

// GetStore 获取全局认证仓库单例
func GetStore() *Store {
	onceStore.Do(func() {
		defaultStore = &Store{
			authDir:  "auths",
			accounts: make(map[string]*WorkBuddyAuth),
		}
	})
	return defaultStore
}

// Init 初始化并指定凭据存储目录
func (s *Store) Init(authDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if authDir != "" {
		s.authDir = authDir
	}
	return s.loadAllLocked()
}

// loadAllLocked 从 authDir 扫描并加载所有 workbuddy 凭据文件
func (s *Store) loadAllLocked() error {
	if _, err := os.Stat(s.authDir); os.IsNotExist(err) {
		return os.MkdirAll(s.authDir, 0755)
	}

	entries, err := os.ReadDir(s.authDir)
	if err != nil {
		return fmt.Errorf("read auth dir failed: %w", err)
	}

	loaded := make(map[string]*WorkBuddyAuth)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "workbuddy") {
			continue
		}

		fullPath := filepath.Join(s.authDir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		var acc WorkBuddyAuth
		if err := json.Unmarshal(data, &acc); err != nil {
			continue
		}

		if acc.Type != "workbuddy" && acc.Type != "" {
			continue
		}
		acc.Type = "workbuddy"
		acc.FilePath = fullPath

		if acc.Realm == "" {
			if strings.Contains(entry.Name(), "intl") || strings.Contains(entry.Name(), "global") {
				acc.Realm = RealmGlobal
			} else {
				acc.Realm = RealmCN
			}
		}

		if acc.ID == "" {
			acc.ID = strings.TrimSuffix(entry.Name(), ".json")
		}

		loaded[acc.ID] = &acc
	}

	s.accounts = loaded
	return nil
}

// Reload 重新从磁盘扫描加载
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadAllLocked()
}

// List 获取所有账号快照
func (s *Store) List() []*WorkBuddyAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*WorkBuddyAuth, 0, len(s.accounts))
	for _, acc := range s.accounts {
		list = append(list, acc.Copy())
	}
	return list
}

// ListByRealm 按区域筛选有效可用账号
func (s *Store) ListByRealm(realm Realm, includeDisabled bool) []*WorkBuddyAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*WorkBuddyAuth
	for _, acc := range s.accounts {
		if acc.Realm == realm {
			if !includeDisabled && acc.Disabled {
				continue
			}
			list = append(list, acc.Copy())
		}
	}
	return list
}

// GetByID 根据凭证 ID 获取凭证对象
func (s *Store) GetByID(id string) *WorkBuddyAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if acc, ok := s.accounts[id]; ok {
		return acc.Copy()
	}
	return nil
}

// Save 更新并原子写回磁盘
func (s *Store) Save(acc *WorkBuddyAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acc.ID == "" {
		return fmt.Errorf("account ID cannot be empty")
	}

	if acc.FilePath == "" {
		acc.FilePath = filepath.Join(s.authDir, fmt.Sprintf("%s.json", acc.ID))
	}

	data, err := json.MarshalIndent(acc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal account failed: %w", err)
	}

	tmpFile := acc.FilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp auth file failed: %w", err)
	}

	if err := os.Rename(tmpFile, acc.FilePath); err != nil {
		return fmt.Errorf("atomic rename auth file failed: %w", err)
	}

	s.accounts[acc.ID] = acc.Copy()
	return nil
}

// Delete 从内存及磁盘删除凭据文件
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[id]
	if !ok {
		return nil
	}

	if acc.FilePath != "" {
		_ = os.Remove(acc.FilePath)
	}
	delete(s.accounts, id)
	return nil
}
