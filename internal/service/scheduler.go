package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待执行
	TaskStatusRunning   TaskStatus = "running"   // 执行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 执行失败
	TaskStatusCancelled TaskStatus = "cancelled" // 已取消
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	TaskPriorityLow    TaskPriority = 0 // 低优先级
	TaskPriorityNormal TaskPriority = 1 // 普通优先级
	TaskPriorityHigh   TaskPriority = 2 // 高优先级
	TaskPriorityUrgent TaskPriority = 3 // 紧急优先级
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeOneTime    TaskType = "one_time"     // 一次性任务
	TaskTypePeriodic   TaskType = "periodic"     // 周期性任务
	TaskTypeDelayed    TaskType = "delayed"      // 延迟任务
	TaskTypeRecurring  TaskType = "recurring"    // 重复性任务
)

// Task 任务接口
type Task interface {
	Execute(ctx context.Context) error
	GetID() string
	GetType() TaskType
	GetStatus() TaskStatus
	GetPriority() TaskPriority
	GetSchedule() string
	GetNextRunTime() time.Time
	GetLastRunTime() *time.Time
	GetError() string
}

// BaseTask 基础任务结构
type BaseTask struct {
	ID           string       `json:"id"`
	Type         TaskType     `json:"type"`
	Status       TaskStatus   `json:"status"`
	Priority     TaskPriority `json:"priority"`
	Schedule     string       `json:"schedule"`
	NextRunTime  time.Time    `json:"next_run_time"`
	LastRunTime  *time.Time   `json:"last_run_time,omitempty"`
	Error        string       `json:"error,omitempty"`
	RetryCount   int          `json:"retry_count"`
	MaxRetries   int          `json:"max_retries"`
	Timeout      time.Duration `json:"timeout"`
	Data         map[string]interface{} `json:"data,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxConcurrentTasks int           // 最大并发任务数
	TaskTimeout        time.Duration // 任务超时时间
	RetryInterval      time.Duration // 重试间隔
	MaxRetries         int           // 最大重试次数
	CleanupInterval    time.Duration // 清理间隔
	QueueSize          int           // 队列大小
}

// Scheduler 调度器
type Scheduler struct {
	config       *SchedulerConfig
	logger       *Logger
	tasks        map[string]Task
	queue        chan Task
	mu           sync.RWMutex
	stopCh       chan struct{}
	workerPool   chan struct{}
}

// NewScheduler 创建调度器实例
func NewScheduler(config *SchedulerConfig, logger *Logger) *Scheduler {
	scheduler := &Scheduler{
		config:     config,
		logger:     logger,
		tasks:      make(map[string]Task),
		queue:      make(chan Task, config.QueueSize),
		stopCh:     make(chan struct{}),
		workerPool: make(chan struct{}, config.MaxConcurrentTasks),
	}

	// 启动调度器
	go scheduler.start()

	return scheduler
}

// start 启动调度器
func (s *Scheduler) start() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRunTasks()
		}
	}
}

// checkAndRunTasks 检查并运行任务
func (s *Scheduler) checkAndRunTasks() {
	s.mu.RLock()
	now := time.Now()
	for _, task := range s.tasks {
		if task.GetStatus() == TaskStatusPending && !task.GetNextRunTime().After(now) {
			s.queue <- task
		}
	}
	s.mu.RUnlock()

	// 启动工作协程
	go s.processQueue()
}

// processQueue 处理任务队列
func (s *Scheduler) processQueue() {
	for task := range s.queue {
		// 获取工作协程
		s.workerPool <- struct{}{}

		go func(t Task) {
			defer func() { <-s.workerPool }()

			// 更新任务状态
			s.mu.Lock()
			if baseTask, ok := t.(*BaseTask); ok {
				baseTask.Status = TaskStatusRunning
				baseTask.UpdatedAt = time.Now()
			}
			s.mu.Unlock()

			// 创建上下文
			ctx, cancel := context.WithTimeout(context.Background(), s.config.TaskTimeout)
			defer cancel()

			// 执行任务
			err := t.Execute(ctx)

			// 更新任务状态
			s.mu.Lock()
			if baseTask, ok := t.(*BaseTask); ok {
				now := time.Now()
				baseTask.LastRunTime = &now
				baseTask.UpdatedAt = now

				if err != nil {
					baseTask.Error = err.Error()
					baseTask.RetryCount++
					if baseTask.RetryCount < baseTask.MaxRetries {
						baseTask.Status = TaskStatusPending
						baseTask.NextRunTime = now.Add(s.config.RetryInterval)
					} else {
						baseTask.Status = TaskStatusFailed
					}
				} else {
					baseTask.Status = TaskStatusCompleted
					baseTask.Error = ""
				}
			}
			s.mu.Unlock()
		}(task)
	}
}

// ScheduleTask 调度任务
func (s *Scheduler) ScheduleTask(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查任务是否已存在
	if _, exists := s.tasks[task.GetID()]; exists {
		return fmt.Errorf("task already exists: %s", task.GetID())
	}

	// 设置任务状态
	if baseTask, ok := task.(*BaseTask); ok {
		baseTask.Status = TaskStatusPending
		baseTask.CreatedAt = time.Now()
		baseTask.UpdatedAt = baseTask.CreatedAt
	}

	// 保存任务
	s.tasks[task.GetID()] = task

	return nil
}

// SchedulePeriodicTask 调度周期性任务
func (s *Scheduler) SchedulePeriodicTask(task Task, interval time.Duration) error {
	if baseTask, ok := task.(*BaseTask); ok {
		baseTask.Type = TaskTypePeriodic
		baseTask.NextRunTime = time.Now().Add(interval)
	}
	return s.ScheduleTask(task)
}

// ScheduleDelayedTask 调度延迟任务
func (s *Scheduler) ScheduleDelayedTask(task Task, delay time.Duration) error {
	if baseTask, ok := task.(*BaseTask); ok {
		baseTask.Type = TaskTypeDelayed
		baseTask.NextRunTime = time.Now().Add(delay)
	}
	return s.ScheduleTask(task)
}

// CancelTask 取消任务
func (s *Scheduler) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if baseTask, ok := task.(*BaseTask); ok {
		baseTask.Status = TaskStatusCancelled
		baseTask.UpdatedAt = time.Now()
	}

	return nil
}

// GetTask 获取任务
func (s *Scheduler) GetTask(taskID string) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListTasks 获取任务列表
func (s *Scheduler) ListTasks(status TaskStatus, taskType TaskType) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []Task
	for _, task := range s.tasks {
		if (status == "" || task.GetStatus() == status) &&
			(taskType == "" || task.GetType() == taskType) {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// GetStats 获取调度器统计信息
func (s *Scheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tasks":     len(s.tasks),
		"pending":         0,
		"running":         0,
		"completed":       0,
		"failed":          0,
		"cancelled":       0,
		"by_type":         make(map[TaskType]int),
		"by_priority":     make(map[TaskPriority]int),
		"queue_size":      len(s.queue),
		"worker_count":    len(s.workerPool),
	}

	for _, task := range s.tasks {
		// 统计状态
		switch task.GetStatus() {
		case TaskStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case TaskStatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case TaskStatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case TaskStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case TaskStatusCancelled:
			stats["cancelled"] = stats["cancelled"].(int) + 1
		}

		// 统计类型
		stats["by_type"].(map[TaskType]int)[task.GetType()]++

		// 统计优先级
		stats["by_priority"].(map[TaskPriority]int)[task.GetPriority()]++
	}

	return stats
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	close(s.stopCh)
	close(s.queue)
} 