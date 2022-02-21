package cur_render

import (
	"errors"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"k8s.io/apimachinery/pkg/runtime"
)

func getContextString(object runtime.Object) (string, error) {
	kind := object.GetObjectKind().GroupVersionKind().Kind
	if len(kind) == 0 {
		return "", errors.New("kind is empty")
	}
	ctx := cuecontext.New()
	value := ctx.Encode(object)
	contextString := fmt.Sprintln(value)
	return "context: " + contextString, nil
}

func RenderCue(object runtime.Object, cueString, parsePath string) ([]byte, error) {
	contextString, err := getContextString(object)
	if err != nil {
		return nil, err
	}

	ctx := cuecontext.New()
	allValue := ctx.CompileString(cueString + contextString)
	err = allValue.Err()
	if err != nil {
		return nil, err
	}

	if len(parsePath) == 0 {
		parsePath = "output"
	}

	patchValue := allValue.LookupPath(cue.ParsePath(parsePath))
	err = patchValue.Err()
	if err != nil {
		return nil, err
	}
	valueByte, err := patchValue.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return valueByte, nil
}
