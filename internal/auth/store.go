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
		absPath, err := filepath.Abs(authDir)
		if err == nil {
			s.authDir = absPath
		} else {
			s.authDir = authDir
		}
	}
	return s.loadAllLocked()
}

// loadAllLocked 从 authDir 及候选迁移目录扫描并加载所有凭据文件 (支持 workbuddy*, codebuddy*, copilot*)
func (s *Store) loadAllLocked() error {
	if s.authDir == "" {
		s.authDir = "auths"
	}
	if _, err := os.Stat(s.authDir); os.IsNotExist(err) {
		_ = os.MkdirAll(s.authDir, 0755)
	}

	loaded := make(map[string]*WorkBuddyAuth)

	// 1. 扫描当前主 authDir 目录 (应为 CPA auth-dir 下的 workbuddy/ 子目录)
	s.scanDirInto(s.authDir, loaded)

	// 2. 扫描历史/迁移目录 (CPA 根目录, wb-auths, 相对 ./auths)
	//    注意：CPA 宿主会扫描 auth-dir 根目录下的 *.json 并当作自家凭据加载，
	//    插件凭据若留在根目录会导致 CPAMP 中每个账号出现重复条目；
	//    因此发现根目录中的本插件凭据文件时，自动迁移到子目录并清理根目录副本。
	candidateDirs := []string{
		"wb-auths",
		"auths",
		filepath.Dir(s.authDir),
	}

	for _, cand := range candidateDirs {
		if cand == "" {
			continue
		}
		absCand, errCand := filepath.Abs(cand)
		if errCand != nil || absCand == s.authDir {
			continue
		}

		oldAccounts := make(map[string]*WorkBuddyAuth)
		s.scanDirInto(absCand, oldAccounts)

		// 将旧目录发现的有效账号自动载入并迁移至主 authDir，并清理旧位置副本避免 CPA 重复扫描
		for id, acc := range oldAccounts {
			if existing, exists := loaded[id]; exists {
				// 已存在于主目录：直接删除旧位置的重复副本
				if acc.FilePath != "" && existing.FilePath != "" && acc.FilePath != existing.FilePath {
					_ = os.Remove(acc.FilePath)
				}
				continue
			}
			loaded[id] = acc
			oldPath := acc.FilePath
			acc.FilePath = filepath.Join(s.authDir, fmt.Sprintf("%s.json", acc.ID))
			// 先写盘迁移成功，再清理旧文件，保证数据安全
			if err := s.saveAccountToDisk(acc); err == nil && oldPath != "" && oldPath != acc.FilePath {
				_ = os.Remove(oldPath)
			}
		}
	}

	s.accounts = loaded
	return nil
}

func (s *Store) scanDirInto(dir string, target map[string]*WorkBuddyAuth) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		nameLower := strings.ToLower(entry.Name())
		if !strings.HasPrefix(nameLower, "workbuddy") &&
			!strings.HasPrefix(nameLower, "codebuddy") &&
			!strings.HasPrefix(nameLower, "copilot") {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		parsedList, err := ParseAuthPayload(data, entry.Name(), fullPath)
		if err != nil || len(parsedList) == 0 {
			continue
		}

		for _, acc := range parsedList {
			target[acc.ID] = acc
		}
	}
}

// StoredAuthCodeBuddy 兼容 Sliverkiss / 官方 workbuddy.json 存储结构
type StoredAuthCodeBuddy struct {
	Auth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"`
		Domain       string `json:"domain"`
	} `json:"auth"`
	Account struct {
		UID          string `json:"uid"`
		EnterpriseID string `json:"enterpriseId"`
		Nickname     string `json:"nickname"`
	} `json:"account"`
}

// ParseAuthPayload 解析一段 JSON 数据，支持单账号、多账号数组、CodeBuddy 结构以及 WorkBuddy 结构
func ParseAuthPayload(raw []byte, fileName, path string) ([]*WorkBuddyAuth, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty auth payload")
	}

	// 1. 尝试解析为顶层数组 (多账号格式 [ {...}, {...} ])
	var rawList []json.RawMessage
	if err := json.Unmarshal(raw, &rawList); err == nil && len(rawList) > 0 {
		var results []*WorkBuddyAuth
		for idx, item := range rawList {
			accs, err := parseSingleOrNested(item, fmt.Sprintf("%s#%d", fileName, idx), path)
			if err == nil {
				results = append(results, accs...)
			}
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// 2. 尝试解析为 {"accounts": [ ... ]}
	var wrapped struct {
		Accounts []json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Accounts) > 0 {
		var results []*WorkBuddyAuth
		for idx, item := range wrapped.Accounts {
			accs, err := parseSingleOrNested(item, fmt.Sprintf("%s#%d", fileName, idx), path)
			if err == nil {
				results = append(results, accs...)
			}
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// 3. 单账号解析
	return parseSingleOrNested(raw, fileName, path)
}

func parseSingleOrNested(raw []byte, fileName, path string) ([]*WorkBuddyAuth, error) {
	// A. 尝试 CodeBuddy standard format
	var cb StoredAuthCodeBuddy
	if err := json.Unmarshal(raw, &cb); err == nil && cb.Auth.AccessToken != "" {
		realm := SniffRealm(cb.Auth.Domain, fileName, path)
		accID := generateAccountID(cb.Account.UID, realm, fileName)
		acc := &WorkBuddyAuth{
			Type:         "workbuddy",
			ID:           accID,
			Realm:        realm,
			AccessToken:  cb.Auth.AccessToken,
			RefreshToken: cb.Auth.RefreshToken,
			ExpiresAt:    cb.Auth.ExpiresAt,
			UserID:       cb.Account.UID,
			EnterpriseID: cb.Account.EnterpriseID,
			Nickname:     cb.Account.Nickname,
			FilePath:     path,
		}
		return []*WorkBuddyAuth{acc}, nil
	}

	// B. 尝试 WorkBuddy 原生结构
	var wb WorkBuddyAuth
	if err := json.Unmarshal(raw, &wb); err == nil && wb.AccessToken != "" {
		if wb.Realm == "" {
			wb.Realm = SniffRealm("", fileName, path)
		}
		if wb.ID == "" {
			wb.ID = generateAccountID(wb.UserID, wb.Realm, fileName)
		}
		wb.Type = "workbuddy"
		wb.FilePath = path
		return []*WorkBuddyAuth{&wb}, nil
	}

	return nil, fmt.Errorf("unrecognized auth format")
}

// SniffRealm 根据域名、文件名与路径推断账号归属区域 (RealmCN vs RealmGlobal)
func SniffRealm(domain, fileName, path string) Realm {
	combined := strings.ToLower(fmt.Sprintf("%s %s %s", domain, fileName, path))

	if strings.Contains(combined, "copilot.tencent.com") ||
		strings.Contains(combined, "codebuddy.cn") ||
		strings.Contains(combined, "workbuddy.cn") ||
		strings.Contains(combined, "-cn") ||
		strings.Contains(combined, "_cn") ||
		strings.Contains(combined, ".cn.") {
		return RealmCN
	}

	if strings.Contains(combined, "codebuddy.ai") ||
		strings.Contains(combined, "workbuddy.ai") ||
		strings.Contains(combined, "intl") ||
		strings.Contains(combined, "global") ||
		strings.Contains(combined, "-en") ||
		strings.Contains(combined, "_en") {
		return RealmGlobal
	}

	// 默认依据文件名推断
	if strings.Contains(strings.ToLower(fileName), "cn") {
		return RealmCN
	}
	if strings.Contains(strings.ToLower(fileName), "global") || strings.Contains(strings.ToLower(fileName), "intl") {
		return RealmGlobal
	}

	// 兜底默认设为 CN
	return RealmCN
}

func generateAccountID(uid string, realm Realm, fileName string) string {
	cleanName := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	if uid != "" {
		return fmt.Sprintf("workbuddy-%s-%s", uid, realm)
	}
	if cleanName != "" && cleanName != "workbuddy" {
		return cleanName
	}
	return fmt.Sprintf("workbuddy-%s", realm)
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

// FindAccount 智能多模式匹配账号 (精确匹配、去前缀匹配、UID 匹配、包含关系与文件名检索)
func (s *Store) FindAccount(identifier string) *WorkBuddyAuth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id := strings.TrimSpace(identifier)
	if id == "" {
		return nil
	}

	// 1. 精确匹配
	if acc, ok := s.accounts[id]; ok {
		return acc.Copy()
	}

	// 2. 清理常见前缀后匹配 (去除 provider 前缀 "workbuddy/", "workbuddyai/", "codebuddy/")
	cleanID := id
	for _, p := range []string{"workbuddy/", "workbuddyai/", "codebuddy/", "workbuddy-", "workbuddyai-"} {
		cleanID = strings.TrimPrefix(cleanID, p)
	}
	cleanID = strings.TrimSuffix(cleanID, ".json")

	if acc, ok := s.accounts[cleanID]; ok {
		return acc.Copy()
	}

	// 3. 遍历账号池按 UID、包含关系或文件名模糊检索
	for _, acc := range s.accounts {
		// A. UID 包含或匹配
		if acc.UserID != "" && (acc.UserID == id || acc.UserID == cleanID || strings.Contains(id, acc.UserID)) {
			return acc.Copy()
		}
		// B. ID 包含关系 (例如 workbuddy-da4c1fb7... 与 workbuddy-da4c1fb7...-global)
		if acc.ID == cleanID || strings.Contains(id, acc.ID) || strings.Contains(acc.ID, cleanID) {
			return acc.Copy()
		}
		// C. 文件名匹配
		if acc.FilePath != "" {
			base := filepath.Base(acc.FilePath)
			cleanBase := strings.TrimSuffix(base, filepath.Ext(base))
			if base == id || cleanBase == id || cleanBase == cleanID || strings.Contains(id, cleanBase) {
				return acc.Copy()
			}
		}
	}

	return nil
}

// UpsertInMemory 仅在内存中注册或更新账号 (供插件 RPC 动态注册)
func (s *Store) UpsertInMemory(acc *WorkBuddyAuth) {
	if acc == nil || acc.ID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[acc.ID] = acc.Copy()
}

// Save 更新并原子写回磁盘
func (s *Store) Save(acc *WorkBuddyAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acc.ID == "" {
		return fmt.Errorf("account ID cannot be empty")
	}

	if acc.FilePath == "" || !strings.HasPrefix(acc.FilePath, s.authDir) {
		acc.FilePath = filepath.Join(s.authDir, fmt.Sprintf("%s.json", acc.ID))
	}

	if err := s.saveAccountToDisk(acc); err != nil {
		return err
	}

	s.accounts[acc.ID] = acc.Copy()
	return nil
}

func (s *Store) saveAccountToDisk(acc *WorkBuddyAuth) error {
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
