package usecases

import (
	"context"
	"gin-quickstart/internal/domain/entity"
	"gin-quickstart/internal/domain/ports"
	repository "gin-quickstart/internal/infra/repository/memory"
	"gin-quickstart/pkg/json"
	"io"
	"net/http"
	"time"
)

type DownloadUseCase struct {
	DownloadJobRepository ports.DownloadJobRepository
	FileRepository        ports.FileRepository
}

func NewDownloadUseCase() *DownloadUseCase {
	return &DownloadUseCase{
		DownloadJobRepository: repository.NewDownloadJobMemoryRepository(),
		FileRepository:        repository.NewFileMemoryRepository(),
	}
}

func (u *DownloadUseCase) createJobEntity(duration time.Duration) entity.DownloadJob {
	return entity.DownloadJob{
		Status:    entity.Pending,
		Timeout:   duration,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (u *DownloadUseCase) StartJob(urls []string, duration time.Duration) (entity.DownloadJob, error) {
	job := u.createJobEntity(duration)

	ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
	defer cancel()

	client := &http.Client{}
	sem := make(chan struct{}, 10)
	type res struct {
		fileID string
		err    error
	}
	resCh := make(chan res)

	createdJob, err := u.DownloadJobRepository.Create(job)
	if err != nil {
		return entity.DownloadJob{}, err
	}

	json.PrettyPrint(createdJob)

	for _, url := range urls {

		go func(goURL string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// if timeout, return error
			if err := ctx.Err(); err != nil {
				resCh <- res{err: err}
				return
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, goURL, nil)
			if err != nil {
				resCh <- res{err: err}
				return
			}

			req.Header.Set("User-Agent", "go-school-downloader/1.0 (contact: tarek.fakhfakh@gmail.com)")
			resp, err := client.Do(req)
			if err != nil {
				resCh <- res{err: err}
				return
			}
			defer resp.Body.Close()

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				resCh <- res{err: err}
				return
			}

			file := entity.File{
				Metadata: entity.FileMetadata{
					MimeType: resp.Header.Get("Content-Type"),
					Size:     resp.ContentLength,
				},
				Data: data,
			}

			fileID, err := u.FileRepository.Put(file)
			if err != nil {
				resCh <- res{err: err}
				return
			}

			resCh <- res{fileID: fileID, err: nil}

		}(url)

	}

	job.Status = entity.Running
	_ = u.DownloadJobRepository.Update(job)

	for i := 0; i < len(urls); i++ {
		select {
		case <-ctx.Done():
			i = len(urls)
		case res := <-resCh:
			if res.err != nil && res.fileID == "" {
				createdJob.Items = append(createdJob.Items, entity.DownlaodItem{
					URL:   urls[i],
					Error: &entity.DownloadItemError{Code: entity.ErrorUnknown},
				})
			} else if res.fileID != "" {
				createdJob.Items = append(createdJob.Items, entity.DownlaodItem{
					URL:    urls[i],
					FileID: res.fileID,
				})
			}
		}
	}

	job.Status = entity.Done
	_ = u.DownloadJobRepository.Update(job)

	return createdJob, nil
}

func (u *DownloadUseCase) GetJob(jobID string) (entity.DownloadJob, error) {
	return u.DownloadJobRepository.Get(jobID)
}

func (u *DownloadUseCase) GetFile(jobID, fileID string) (entity.File, error) {
	return u.FileRepository.Get(fileID)
}
