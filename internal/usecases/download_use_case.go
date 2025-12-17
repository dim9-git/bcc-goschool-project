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
	jobEntity := u.createJobEntity(duration)

	ctx, cancel := context.WithTimeout(context.Background(), jobEntity.Timeout)
	defer cancel()

	client := &http.Client{}
	sem := make(chan struct{}, 10)
	type res struct {
		url    string
		fileID string
		err    error
	}
	resCh := make(chan res)

	job, err := u.DownloadJobRepository.Create(jobEntity)
	if err != nil {
		return entity.DownloadJob{}, err
	}

	json.PrettyPrint(job)

	for _, url := range urls {

		go func(goURL string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// if timeout, return error
			if err := ctx.Err(); err != nil {
				resCh <- res{url: goURL, err: err}
				return
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, goURL, nil)
			if err != nil {
				resCh <- res{url: goURL, err: err}
				return
			}

			req.Header.Set("User-Agent", "go-school-downloader/1.0 (contact: tarek.fakhfakh@gmail.com)")
			resp, err := client.Do(req)
			if err != nil {
				resCh <- res{url: goURL, err: err}
				return
			}
			defer resp.Body.Close()

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				resCh <- res{url: goURL, err: err}
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
				resCh <- res{url: goURL, err: err}
				return
			}

			resCh <- res{url: goURL, fileID: fileID, err: nil}

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
				job.Items = append(job.Items, entity.DownlaodItem{
					URL:   res.url,
					Error: &entity.DownloadItemError{Code: entity.ErrorUnknown},
				})
			} else if res.fileID != "" {
				job.Items = append(job.Items, entity.DownlaodItem{
					URL:    res.url,
					FileID: res.fileID,
				})
			}
		}
	}

	job.Status = entity.Done
	_ = u.DownloadJobRepository.Update(job)

	return job, nil
}

func (u *DownloadUseCase) GetJob(jobID string) (entity.DownloadJob, error) {
	return u.DownloadJobRepository.Get(jobID)
}

func (u *DownloadUseCase) GetFile(jobID, fileID string) (entity.File, error) {
	return u.FileRepository.Get(fileID)
}
