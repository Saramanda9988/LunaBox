package plugin

import (
	"context"
	"sync"
)

// Disposer 是幂等的撤销函数。调用一次后从所属作用域移除，再次调用为空操作。
type Disposer func()

type effectEntry struct {
	id      uint64
	dispose func()
}

// Scope 是借鉴 Cordis 的可撤销作用域：插件或插槽实例登记的一切副作用（元数据源、事件监听、插槽、
// 定时器、子窗口、后台请求等）都挂在这里，Dispose 时按后注册先撤销（LIFO）顺序一次清空。
type Scope struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	nextID   uint64
	effects  []*effectEntry
	children []*Scope
	parent   *Scope
	disposed bool
}

// NewScope 创建根作用域。parentCtx 为 nil 时使用 context.Background()。
func NewScope(parentCtx context.Context) *Scope {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	return &Scope{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context 返回随作用域取消的 context。
func (s *Scope) Context() context.Context {
	if s == nil {
		return context.Background()
	}
	return s.ctx
}

// Disposed 报告该作用域是否已经撤销。
func (s *Scope) Disposed() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disposed
}

// Effect 登记一个撤销函数，并返回幂等的 Disposer（可提前单独撤销）。
// 若作用域已撤销，则立即调用 dispose 并返回空 Disposer。
func (s *Scope) Effect(dispose func()) Disposer {
	if dispose == nil {
		return func() {}
	}
	if s == nil {
		var once sync.Once
		return func() { once.Do(dispose) }
	}

	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		safeInvokeDispose(dispose)
		return func() {}
	}
	s.nextID++
	entry := &effectEntry{id: s.nextID, dispose: dispose}
	s.effects = append(s.effects, entry)
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.removeEffect(entry.id)
			safeInvokeDispose(entry.dispose)
		})
	}
}

func (s *Scope) removeEffect(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.effects {
		if e.id == id {
			s.effects = append(s.effects[:i], s.effects[i+1:]...)
			return
		}
	}
}

// Fork 创建子作用域（例如一个插槽实例），父作用域撤销时子作用域自动先于父级副作用撤销。
func (s *Scope) Fork() *Scope {
	if s == nil {
		return NewScope(context.Background())
	}
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		child := NewScope(s.ctx)
		child.Dispose()
		return child
	}
	ctx, cancel := context.WithCancel(s.ctx)
	child := &Scope{
		ctx:    ctx,
		cancel: cancel,
		parent: s,
	}
	s.children = append(s.children, child)
	s.mu.Unlock()
	return child
}

func (s *Scope) detachChild(child *Scope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.children {
		if c == child {
			s.children = append(s.children[:i], s.children[i+1:]...)
			return
		}
	}
}

// Dispose 幂等地取消作用域 context，并按 LIFO 顺序撤销所有子作用域和副作用。
// 撤销函数不能阻塞，即使发生 panic 也会被捕获，保证其余清理继续完成。
func (s *Scope) Dispose() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	s.disposed = true
	children := s.children
	s.children = nil
	effects := s.effects
	s.effects = nil
	parent := s.parent
	s.parent = nil
	s.mu.Unlock()

	if parent != nil {
		parent.detachChild(s)
	}

	s.cancel()

	// 先 LIFO 撤销子作用域
	for i := len(children) - 1; i >= 0; i-- {
		children[i].Dispose()
	}

	// 再 LIFO 执行自身副作用
	for i := len(effects) - 1; i >= 0; i-- {
		safeInvokeDispose(effects[i].dispose)
	}
}

func safeInvokeDispose(fn func()) {
	if fn == nil {
		return
	}
	defer func() { _ = recover() }()
	fn()
}
