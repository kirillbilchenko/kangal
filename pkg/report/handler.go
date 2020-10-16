package report

import (
	"fmt"
	"io/ioutil"
	"net/http"

	khttp "github.com/hellofresh/kangal/pkg/core/http"
	kk8s "github.com/hellofresh/kangal/pkg/kubernetes"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"k8s.io/apimachinery/pkg/api/errors"
)

var httpClient = &http.Client{}

//ShowHandler method returns response from file bucket in defined object storage
func ShowHandler() func(w http.ResponseWriter, r *http.Request) {
	if minioClient == nil {
		panic("client was not initialized, please initialize object storage client")
	}
	if bucketName == "" {
		panic("bucket name was not defined or empty")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		loadTestName := chi.URLParam(r, "id")
		file := chi.URLParam(r, "*")

		r.URL.Path = fmt.Sprintf("/%s", loadTestName)
		if file != "" {
			r.URL.Path += fmt.Sprintf("/%s", file)
		}

		http.FileServer(fs).ServeHTTP(w, r)
	}
}

//PersistHandler method streams request to storage presigned URL
func PersistHandler(kubeClient *kk8s.Client) func(w http.ResponseWriter, r *http.Request) {
	if minioClient == nil {
		panic("client was not initialized, please initialize object storage client")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		loadTestName := chi.URLParam(r, "id")

		_, err := kubeClient.GetLoadTest(r.Context(), loadTestName)
		if errors.IsNotFound(err) {
			render.Render(w, r, khttp.ErrResponse(http.StatusNotFound, err.Error()))
			return
		}
		if err != nil {
			render.Render(w, r, khttp.ErrResponse(http.StatusInternalServerError, err.Error()))
			return
		}

		url, err := newPreSignedPutURL(loadTestName)
		if nil != err {
			render.Render(w, r, khttp.ErrResponse(http.StatusInternalServerError, err.Error()))
			return
		}

		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, url.String(), r.Body)
		proxyReq.ContentLength = r.ContentLength
		proxyReq.Header = r.Header
		if nil != err {
			render.Render(w, r, khttp.ErrResponse(http.StatusInternalServerError, err.Error()))
			return
		}

		proxyResp, err := httpClient.Do(proxyReq)
		if nil != err {
			render.Render(w, r, khttp.ErrResponse(http.StatusInternalServerError, err.Error()))
			return
		}

		body := "Report persisted"

		if http.StatusOK != proxyResp.StatusCode {
			b, _ := ioutil.ReadAll(proxyResp.Body)
			body = string(b)
		}

		render.Status(r, proxyResp.StatusCode)
		render.JSON(w, r, body)
	}
}
