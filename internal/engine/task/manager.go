package task

import (
	"SysInitPro/internal/models"
	"context"
	"fmt"
	"sync"
)

type Manager struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	status  models.TaskStatus
}

func NewManager() *Manager {
	return &Manager{status: models.TaskStatus{State: "idle"}}
}

func (m *Manager) Status() models.TaskStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) Run(id, name string, fn func(ctx context.Context) error) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("已有任务正在运行: %s，请等待完成或先中止", m.status.Name)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running = true
	m.cancel = cancel
	m.status = models.TaskStatus{ID: id, Name: name, State: "running", Message: "执行中..."}
	m.mu.Unlock()

	err := fn(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.cancel = nil
	if ctx.Err() == context.Canceled {
		m.status.State = "canceled"
		m.status.Message = "任务已手动中止"
		return fmt.Errorf("任务已中止")
	}
	if err != nil {
		m.status.State = "failed"
		m.status.Message = err.Error()
		return err
	}
	m.status.State = "success"
	m.status.Message = "执行完成"
	return nil
}

func (m *Manager) Cancel() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}
