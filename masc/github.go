//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall/js"
)

const githubGraphQLEndpoint = "https://api.github.com/graphql"

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type GraphQLClient struct {
	Token string
}

func (c *GraphQLClient) Query(query string, variables map[string]interface{}, out interface{}) error {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	headers := map[string]interface{}{
		"Content-Type": "application/json",
		"Accept":       "application/vnd.github+json",
	}
	if c.Token != "" {
		headers["Authorization"] = "Bearer " + c.Token
	}

	options := map[string]interface{}{
		"method":  "POST",
		"headers": headers,
		"body":    string(body),
	}

	respValue, err := awaitPromise(js.Global().Call("fetch", githubGraphQLEndpoint, js.ValueOf(options)))
	if err != nil {
		return err
	}

	ok := respValue.Get("ok").Bool()
	textValue, err := awaitPromise(respValue.Call("text"))
	if err != nil {
		return err
	}
	text := textValue.String()
	if !ok {
		status := respValue.Get("status").Int()
		message := fmt.Sprintf("graphql status %d: %s", status, strings.TrimSpace(text))
		if status == 401 {
			return newUnauthorizedError(message)
		}
		return fmt.Errorf(message)
	}

	var envelope graphQLResponse
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		message := envelope.Errors[0].Message
		if isUnauthorizedMessage(message) {
			return newUnauthorizedError(message)
		}
		return errors.New(message)
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

func (c *GraphQLClient) REST(method, url string, payload map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	headers := map[string]interface{}{
		"Content-Type": "application/json",
		"Accept":       "application/vnd.github+json",
	}
	if c.Token != "" {
		headers["Authorization"] = "Bearer " + c.Token
	}

	options := map[string]interface{}{
		"method":  method,
		"headers": headers,
		"body":    string(body),
	}

	respValue, err := awaitPromise(js.Global().Call("fetch", url, js.ValueOf(options)))
	if err != nil {
		return nil, err
	}

	ok := respValue.Get("ok").Bool()
	textValue, err := awaitPromise(respValue.Call("text"))
	if err != nil {
		return nil, err
	}
	text := textValue.String()
	if !ok {
		status := respValue.Get("status").Int()
		message := fmt.Sprintf("rest status %d: %s", status, strings.TrimSpace(text))
		if status == 401 {
			return nil, newUnauthorizedError(message)
		}
		return nil, fmt.Errorf(message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func awaitPromise(promise js.Value) (js.Value, error) {
	thenChan := make(chan js.Value, 1)
	errChan := make(chan js.Value, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		thenChan <- args[0]
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		errChan <- args[0]
		return nil
	})
	promise.Call("then", then).Call("catch", catch)
	defer then.Release()
	defer catch.Release()

	select {
	case value := <-thenChan:
		return value, nil
	case errVal := <-errChan:
		if errVal.Type() == js.TypeString {
			return js.Value{}, errors.New(errVal.String())
		}
		if errVal.Get("message").Type() == js.TypeString {
			return js.Value{}, errors.New(errVal.Get("message").String())
		}
		return js.Value{}, errors.New("promise rejected")
	}
}
