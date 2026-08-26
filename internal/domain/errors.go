package domain

import "errors"

var (
	ErrNotFound        = errors.New("案卷不存在")
	ErrConflict        = errors.New("案卷版本冲突")
	ErrInvalidState    = errors.New("当前状态不允许此操作")
	ErrAlreadyApproved = errors.New("案卷已经批准，不能重复操作")
	ErrValidation      = errors.New("输入校验失败")
	ErrForbidden       = errors.New("当前身份无权执行此操作")
	ErrIdempotency     = errors.New("Idempotency-Key 已用于不同命令")
	ErrTokenNotFound   = errors.New("批准确认令牌不存在")
	ErrTokenExpired    = errors.New("批准确认令牌已失效，请重新预览")
	ErrTokenUsed       = errors.New("批准确认令牌已使用")
	ErrTokenMismatch   = errors.New("批准确认令牌与当前案卷、版本、操作者或摘要不一致，请重新预览")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e FieldError) Error() string { return e.Field + ": " + e.Message }

// ValidationErrors 用于需要一次返回全部行级或条目级错误的原子批量命令。
type ValidationErrors struct {
	Items []FieldError `json:"items"`
}

func (e ValidationErrors) Error() string {
	if len(e.Items) == 0 {
		return ErrValidation.Error()
	}
	return e.Items[0].Error()
}
