package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Prev/HotFunctions/worker_front/types"
)

// RequestHandler of the worker front
type RequestHandler struct {
	http.Handler
	functionRunner *FunctionRunner
}

func newRequestHandler(options CachingOptions) *RequestHandler {
	h := new(RequestHandler)
	h.functionRunner = newFunctionRunner(options)
	return h
}

type FailResponse struct {
	Error   bool
	Message string
}

type ConfigureSuccessResponse struct {
	Message string
}

func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger.Println(req.Method, req.URL.String())
	w.Header().Add("Content-Type", "application/json")

	switch req.URL.Path {
	case "/configure":
		h.ConfigureWorker(&w, req)
	case "/clear":
		h.Clear(&w, req)
	case "/prepare":
		h.Prepare(&w, req)
	case "/execute":
		h.ExecFunction(&w, req)
	default:
		w.WriteHeader(404)
		writeFailResponse(&w, "404 Not found on given path")
	}
}

func (h *RequestHandler) ConfigureWorker(w *http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	oldOptions := h.functionRunner.cachingOptions

	if v := q["user_code_size_limit"]; len(v) > 0 {
		val, _ := strconv.ParseInt(v[0], 10, 64)
		h.functionRunner.cachingOptions.UserCodeSizeLimit = val
	}
	if v := q["container_pool_limit"]; len(v) > 0 {
		val, _ := strconv.Atoi(v[0])
		h.functionRunner.cachingOptions.ContainerPoolLimit = val
	}
	if v := q["container_pool_num"]; len(v) > 0 {
		val, _ := strconv.Atoi(v[0])
		h.functionRunner.cachingOptions.ContainerPoolNum = val
	}
	if v := q["using_rest_mode"]; len(v) > 0 {
		if v[0] == "true" {
			h.functionRunner.cachingOptions.UsingRestMode = true
		} else if v[0] == "false" {
			h.functionRunner.cachingOptions.UsingRestMode = false
		}
		h.functionRunner.images = make(map[string]Image)
	}
	if v := q["rest_container_life_time"]; len(v) > 0 {
		val, _ := strconv.Atoi(v[0])
		h.functionRunner.cachingOptions.RestContainerLifeTime = val
	}

	message := "nothing changed"
	newOptions := h.functionRunner.cachingOptions
	if oldOptions.UserCodeSizeLimit != newOptions.UserCodeSizeLimit ||
		oldOptions.ContainerPoolLimit != newOptions.ContainerPoolLimit ||
		oldOptions.ContainerPoolNum != newOptions.ContainerPoolNum ||
		oldOptions.UsingRestMode != newOptions.UsingRestMode ||
		oldOptions.RestContainerLifeTime != newOptions.RestContainerLifeTime {
		message = "configure changed"
	}

	resp := ConfigureSuccessResponse{message}
	bytes, _ := json.Marshal(resp)
	(*w).Write(bytes)
}

func (h *RequestHandler) Clear(w *http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	if v := q["reset_images"]; len(v) > 0 {
		if v[0] == "true" {
			h.functionRunner.Reset(true)
			(*w).Write([]byte("done; reset_images=true"))
			return
		}
	}
	h.functionRunner.Reset(false)
	(*w).Write([]byte("done; reset_images=false"))
}

func (h *RequestHandler) Prepare(w *http.ResponseWriter, req *http.Request) {
	err := h.functionRunner.PrepareImages()
	if err != nil {
		(*w).Write([]byte(err.Error()))
	} else {
		(*w).Write([]byte("done"))
	}
}

func (h *RequestHandler) ExecFunction(w *http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	nameParam := q["name"]

	if len(nameParam) == 0 {
		writeFailResponse(w, "param 'name' is not given")
		return
	}
	startTime := makeTimestamp()

	functionName := nameParam[0]
	out, meta := h.functionRunner.RunFunction(functionName)
	if out == nil {
		writeFailResponse(w, "error on running a function")
		return
	}

	endTime := makeTimestamp()
	logger.Println("fin", functionName)

	resp := types.ExecSuccessResponse{
		out.Data,
		endTime - startTime,
		out.EndTime - out.StartTime,
		meta,
	}
	bytes, _ := json.Marshal(resp)
	(*w).Write(bytes)
}

func writeFailResponse(w *http.ResponseWriter, message string) {
	resp := FailResponse{true, message}
	bytes, _ := json.Marshal(resp)
	(*w).WriteHeader(500)
	(*w).Write(bytes)
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
