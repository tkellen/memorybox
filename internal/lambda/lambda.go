package lambda

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/gobuffalo/packr"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type Exec struct {
	client Runner
}

type Runner interface {
	InvokeWithContext(ctx aws.Context, input *lambda.InvokeInput, opts ...request.Option) (*lambda.InvokeOutput, error)
}

const name = "memorybox"

func Run(ctx context.Context, cfg string, args []string, stdin io.Reader) (stdout string, stderr string, code int, err error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return "", "", 1, nil
	}
	return (Exec{
		client: lambda.New(sess),
	}).run(ctx, cfg, args, stdin)
}

func (e Exec) run(ctx context.Context, cfg string, args []string, stdin io.Reader) (stdout string, stderr string, code int, err error) {
	type payload struct {
		Config string   `json:"config"`
		Args   []string `json:"args"`
		Stdin  string   `json:"stdin,omitempty"`
	}
	input, stdinErr := ioutil.ReadAll(stdin)
	if stdinErr != nil {
		return "", "", 1, stdinErr
	}
	jsonPayload, marshalErr := json.Marshal(payload{
		Config: cfg,
		Args:   args,
		Stdin:  string(input),
	})
	if marshalErr != nil {
		return "", "", 1, marshalErr
	}
	// If json payload gets over 6mb (lambda limit) this will need to
	// be broken up into chunks and executed in parallel.
	res, invokeErr := e.client.InvokeWithContext(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(name),
		Payload:      jsonPayload,
	})
	if invokeErr != nil {
		return "", "", 1, invokeErr
	}
	var results map[string]interface{}
	if unmarshalErr := json.Unmarshal(res.Payload, &results); unmarshalErr != nil {
		return "", "", 1, unmarshalErr
	}
	if _, ok := results["errorMessage"]; ok {
		return "", "", 1, fmt.Errorf("%s\n%s\n%s", results["errorType"], results["errorMessage"], results["stackTrace"])
	}
	if stderr, err = read(results, "stderr"); err != nil {
		return "", "", 1, err
	}
	if stdout, err = read(results, "stdout"); err != nil {
		return "", "", 1, err
	}
	returnCode, ok := results["code"].(float64)
	if ok {
		code = int(returnCode)
	} else {
		code = 1
	}
	return stdout, stderr, code, nil
}

func CreateScript(version string) (string, error) {
	box := packr.NewBox("./scripts")
	script, err := box.FindString("create.sh")
	if err != nil {
		return "", err
	}
	run, runErr := box.FindString("run.py")
	if runErr != nil {
		return "", runErr
	}
	script = strings.ReplaceAll(script, "${SCRIPT}", run)
	script = strings.ReplaceAll(script, "${ROLE_NAME}", name)
	script = strings.ReplaceAll(script, "${TEMP_DIR}", os.TempDir())
	script = strings.ReplaceAll(script, "${VERSION}", version)
	return script, nil
}

func DeleteScript() (string, error) {
	box := packr.NewBox("./scripts")
	script, err := box.FindString("delete.sh")
	if err != nil {
		return "", err
	}
	script = strings.ReplaceAll(script, "${ROLE_NAME}", name)
	return script, nil
}

func read(source map[string]interface{}, key string) (string, error) {
	raw, decodeErr := base64.StdEncoding.DecodeString(source[key].(string))
	if decodeErr != nil {
		return "", decodeErr
	}
	b := bytes.NewBuffer(raw)
	r, err := gzip.NewReader(b)
	if err != nil {
		return "", err
	}
	var result bytes.Buffer
	if _, err := result.ReadFrom(r); err != nil {
		return "", err
	}
	return string(result.Bytes()), nil
}
