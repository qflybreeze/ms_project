package repo

import (
	"context"
	"go_project/ms_project/project_user/internal/data/member"
	"go_project/ms_project/project_user/internal/database"
)

type MemberRepo interface {
	GetMemberByEmail(ctx context.Context, email string) (bool, error)
	GetMemberByAccount(ctx context.Context, account string) (bool, error)
	GetMemberByMobile(ctx context.Context, mobile string) (bool, error)
	SaveMember(conn database.DbConn, ctx context.Context, mem *member.Member) error
	// 仅通过账号查询用户，密码校验由 bcrypt 在应用层完成
	FindMemberByAccount(ctx context.Context, account string) (*member.Member, error)
	FindMemberById(ctx context.Context, id int64) (*member.Member, error)
	FindMemberByIds(background context.Context, ids []int64) (list []*member.Member, err error)
	FindAllMemberIds(ctx context.Context) ([]int64, error)
	UpdateMemberPassword(ctx context.Context, id int64, newHash string) error
}
