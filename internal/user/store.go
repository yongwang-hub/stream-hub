// Package user 是「用户」这个业务模块。
//
// 目录内是标准三层：
//
//	store.go     数据层：Store 接口 + memStore 内存实现（map + 读写锁）
//	service.go   业务层：Service，负责校验与编排，只依赖 Store 接口
//	handler.go   接口层：Handler，负责 HTTP 请求/响应翻译
//	dbstore.go   数据层的第二种实现：dbStore（SQLite），与 memStore 互换
//
// 关键：service/handler 只认 Store 接口，不认具体实现。memStore(内存) 与
// dbStore(SQLite) 实现同一接口、返回同样的哨兵错误，装配处一改即可切换；测试可注入 fake。
package user

import (
	"context"
	"errors"
	"sync"
)

// User 是业务实体。`json` 标签决定它序列化后对外暴露的字段名。
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// 数据层哨兵错误：只说明「发生了什么」，怎么对外表达交给上层。
// 注意：参数校验类错误不在这里，那属于业务层（service 的 ValidationError）。
var (
	ErrNotFound      = errors.New("user not found")   // 用户不存在
	ErrDuplicateName = errors.New("user name exists") // 名称重复（业务冲突）
)

// Store 定义「用户存储能做什么」，不关心底层是 map 还是数据库。
// 这就是接口化（DIP：依赖倒置）的落点——上层依赖抽象，不依赖具体实现。
// 每个方法首参统一收 context.Context：数据库实现据此做「请求取消→及时中断慢查询、归还连接」；
// 内存实现用不到就忽略（下划线命名），保持接口一致。
type Store interface {
	Create(ctx context.Context, name, email string) (*User, error)
	List(ctx context.Context) []*User
	Get(ctx context.Context, id int) (*User, error)
	Update(ctx context.Context, id int, name, email string) (*User, error)
	Delete(ctx context.Context, id int) error
}

// memStore 是 Store 的内存实现：用 map 存数据、读写锁保证并发安全。
type memStore struct {
	mu     sync.RWMutex  // 保护下面两个字段的所有读写
	nextID int           // 自增主键
	users  map[int]*User // id -> 用户
}

// NewStore 返回一个内存实现。返回类型刻意写成接口 Store：
// 让调用方从一开始就只看到接口、拿不到具体类型，逼大家面向接口编程。
//
// 返回指针：因为 memStore 内含锁，锁不能被复制，全程必须共用同一个实例。
func NewStore() Store {
	return &memStore{
		nextID: 1,
		users:  make(map[int]*User),
	}
}

// Create 新建用户。名称重复时返回 ErrDuplicateName。
func (s *memStore) Create(_ context.Context, name, email string) (*User, error) {
	s.mu.Lock()         // 写操作：独占锁
	defer s.mu.Unlock() // 函数退出时一定解锁（哪怕中途 return）

	// 重名校验：内存版只能遍历（真实库靠唯一索引，交给数据库判重更快）。
	for _, u := range s.users {
		if u.Name == name {
			return nil, ErrDuplicateName
		}
	}

	u := &User{ID: s.nextID, Name: name, Email: email}
	s.users[s.nextID] = u
	s.nextID++
	return u, nil
}

// List 返回所有用户。
func (s *memStore) List(_ context.Context) []*User {
	s.mu.RLock() // 读操作：共享锁，多个读请求可同时进行
	defer s.mu.RUnlock()

	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

// Get 按 id 取单个用户；不存在返回 ErrNotFound。
func (s *memStore) Get(_ context.Context, id int) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

// Update 更新 name/email；不存在返回 ErrNotFound，与他人重名返回 ErrDuplicateName。
func (s *memStore) Update(_ context.Context, id int, name, email string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}

	// 重名校验，但要排除自己（改自己名字时不算和自己冲突）。
	for _, other := range s.users {
		if other.ID != id && other.Name == name {
			return nil, ErrDuplicateName
		}
	}

	u.Name = name
	u.Email = email
	return u, nil
}

// Delete 删除用户；不存在返回 ErrNotFound。
func (s *memStore) Delete(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}
