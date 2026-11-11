package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type LambdaInvoker struct {
	Endpoint string
	Client   *http.Client
	Signer   Signer
	Now      func() time.Time
	Limits   ConcurrencyLimits
	Entries  EntryRule
}

func (invoker LambdaInvoker) Invoke(caller context.Context, function string, event HttpEvent) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(context.WithoutCancel(caller), http.MethodPost, invoker.Endpoint+"/2015-03-31/functions/"+function+"/invocations", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Amz-Invocation-Type", "RequestResponse")
	release, err := invoker.Entries.For(event).Take(caller, invoker.Limits.Slot(function))
	if err != nil {
		return nil, err
	}
	defer release()
	invoker.Signer.Sign(request, payload, invoker.Now())
	response, err := invoker.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return invoker.checked(function, response, answer)
}

func (invoker LambdaInvoker) checked(function string, response *http.Response, answer []byte) ([]byte, error) {
	failed := response.StatusCode != http.StatusOK || response.Header.Get("X-Amz-Function-Error") != ""
	return invoker.unlessFailed(function, failed, answer, Problems{}.InvokeFailed(response.StatusCode))
}

func (LambdaInvoker) unlessFailed(function string, failed bool, answer []byte, problem Problem) ([]byte, error) {
	if failed {
		slog.Error("function failed", "function", function, "answer", string(answer))
		return nil, problem
	}
	return answer, nil
}
