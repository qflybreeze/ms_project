package model

import (
	"go_project/ms_project/project_common/errs"
)

var (
	NoLegalMobile = errs.NewError(2001, "手机号格式不正确")
)
