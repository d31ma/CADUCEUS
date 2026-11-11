package main

import (
	"net/http"
)

type ApiHandler struct {
	Routes     Routes
	Bodies     BodyReader
	Events     EventBuilder
	Invoker    LambdaInvoker
	Translator ResultTranslator
}

func (handler ApiHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.reply(request).Send(writer)
}

func (handler ApiHandler) reply(request *http.Request) Reply {
	reply, err := handler.invoke(request)
	if err != nil {
		return Problems{}.Reply(err)
	}
	return reply
}

func (handler ApiHandler) invoke(request *http.Request) (Reply, error) {
	function, err := handler.Routes.Function(request.URL.Path)
	if err != nil {
		return Reply{}, err
	}
	body, err := handler.Bodies.Read(request)
	if err != nil {
		return Reply{}, err
	}
	payload, err := handler.Invoker.Invoke(request.Context(), function, handler.Events.Build(request, body))
	if err != nil {
		return Reply{}, err
	}
	return handler.Translator.Reply(payload)
}
