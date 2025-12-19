package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	pkgerrors "gin-quickstart/pkg/errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	validation "github.com/go-ozzo/ozzo-validation"
)

type File struct {
	URL string `json:"url"`
}

type createDownloadJobReq struct {
	Files   []File `json:"files"`
	Timeout string `json:"timeout"`
}

type createDownloadJobResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (req *createDownloadJobReq) Validate() error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.Files, validation.Required),
		validation.Field(&req.Timeout, validation.Required),
	); err != nil {
		var ve validation.Errors
		if errors.As(err, &ve) {
			return pkgerrors.NewValidationErrorFromOzzo(ve)
		}
		return err
	}
	return nil
}

func (h *HTTPHandlers) CreateDownloadJob(w http.ResponseWriter, r *http.Request) {
	var req createDownloadJobReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	duration, err := time.ParseDuration(req.Timeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	urls := make([]string, len(req.Files))
	for i, f := range req.Files {
		urls[i] = f.URL
	}

	rCtx := r.Context()

	createdJob, err := h.DownloadUseCase.StartJob(rCtx, duration, urls)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resDTO := createDownloadJobResp{
		ID:     createdJob.ID,
		Status: createdJob.Status.String(),
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resDTO); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type fileErrorDTO struct {
	Code string `json:"code"`
}

type fileDTO struct {
	URL    string        `json:"url"`
	FileID string        `json:"file_id,omitempty"`
	Error  *fileErrorDTO `json:"error,omitempty"`
}

type jobDTO struct {
	ID     string    `json:"id"`
	Status string    `json:"status"`
	Files  []fileDTO `json:"files"`
}

func (h *HTTPHandlers) GetDownloadJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	fmt.Println("/downloads/{jobID} Job ID: ", jobID)

	rCtx := r.Context()

	job, err := h.DownloadUseCase.GetJob(rCtx, jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respDTO := jobDTO{
		ID:     job.ID,
		Status: job.Status.String(),
		Files:  make([]fileDTO, len(job.Items)),
	}
	for i, item := range job.Items {
		var errDTO *fileErrorDTO
		if item.Error != nil {
			errDTO = &fileErrorDTO{
				Code: string(item.Error.Code),
			}
		}
		respDTO.Files[i] = fileDTO{
			URL:    item.URL,
			FileID: item.FileID,
			Error:  errDTO,
		}
	}

	if err := json.NewEncoder(w).Encode(respDTO); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *HTTPHandlers) GetFile(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	fileID := chi.URLParam(r, "fileID")

	rCtx := r.Context()

	file, err := h.DownloadUseCase.GetFile(rCtx, jobID, fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", file.Metadata.MimeType)
	if file.Metadata.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Metadata.Size, 10))
	}
	w.WriteHeader(http.StatusOK)

	reader := bytes.NewReader(file.Data)

	if _, err := io.Copy(w, reader); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
