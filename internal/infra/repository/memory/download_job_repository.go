package repository

import (
	"context"
	"fmt"
	"gin-quickstart/internal/domain/entity"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DownloadJobMemoryRepository struct {
	mu   sync.RWMutex
	jobs map[string]entity.DownloadJob
}

func NewDownloadJobMemoryRepository() *DownloadJobMemoryRepository {
	return &DownloadJobMemoryRepository{jobs: make(map[string]entity.DownloadJob)}
}

func cloneJob(j entity.DownloadJob) entity.DownloadJob {
	j.Items = append([]entity.DownloadItem(nil), j.Items...)
	return j
}

func (m *DownloadJobMemoryRepository) Create(ctx context.Context, job entity.DownloadJob) (entity.DownloadJob, error) {
	if err := ctx.Err(); err != nil {
		return entity.DownloadJob{}, err
	}

	id := uuid.New().String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[id]; exists {
		return entity.DownloadJob{}, fmt.Errorf("CREATE: Job with ID %s already exists", id)
	}

	now := time.Now()
	job.ID = id
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	m.jobs[job.ID] = cloneJob(job)
	return job, nil
}

func (m *DownloadJobMemoryRepository) Get(ctx context.Context, id string) (entity.DownloadJob, error) {
	if err := ctx.Err(); err != nil {
		return entity.DownloadJob{}, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return entity.DownloadJob{}, fmt.Errorf("GET: Job not found for ID: %s", id)
	}
	return cloneJob(job), nil
}

func (m *DownloadJobMemoryRepository) Update(ctx context.Context, job entity.DownloadJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("UPDATE: Job ID cannot be empty")
	}
	if _, exists := m.jobs[job.ID]; !exists {
		return fmt.Errorf("UPDATE: Job with ID %s not found", job.ID)
	}

	job.UpdatedAt = time.Now()
	m.jobs[job.ID] = cloneJob(job)
	return nil
}

func (m *DownloadJobMemoryRepository) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[id]; !exists {
		return fmt.Errorf("DELETE: Job with ID %s not found", id)
	}
	delete(m.jobs, id)
	return nil
}
