package handler

import (
	"encoding/json"
	"net/http"

	"github.com/basketikun/infinite-canvas/service"
)

func VectorizeImage(w http.ResponseWriter, r *http.Request) {
	var input service.VectorizeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 55<<20)).Decode(&input); err != nil {
		Fail(w, "图片转 SVG 请求格式错误")
		return
	}
	result, err := service.VectorizeImage(input)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, result)
}
