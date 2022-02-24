package cue

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	utils "harmonycloud.cn/stellaris/pkg/util"
)

const (
	ParameterFieldName = "parameters"
)

func Complete(ctx Context, cueTemplate string, params interface{}) (*cue.Value, error) {
	var paramFile = ParameterFieldName + ": {}"
	pb, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("parse params json error: %s", err)
	}
	if string(pb) != "null" {
		paramFile = fmt.Sprintf("%s: %s", ParameterFieldName, string(pb))
	}

	bi := build.NewContext().NewInstance("-", nil)
	if err := utils.ParseAndAddCueFile(bi, "-", fillContext(cueTemplate)); err != nil {
		// if err := utils.ParseAndAddCueFile(bi, "-", cueTemplate); err != nil {
		return nil, fmt.Errorf("parse cue template error: %s", err)
	}
	if err := utils.ParseAndAddCueFile(bi, ParameterFieldName, paramFile); err != nil {
		return nil, fmt.Errorf("parse cue template with params error: %s", err)
	}
	if err := utils.ParseAndAddCueFile(bi, "context", ctx.GetContextFile()); err != nil {
		return nil, fmt.Errorf("add context in cue template error: %s", err)
	}

	inst := cuecontext.New().BuildInstance(bi)
	return &inst, nil
}

// for cue 0.4.0, it can not add not exist in template field in instance
func fillContext(cueTemplate string) string {
	return cueTemplate + "\ncontext: {}"
}
