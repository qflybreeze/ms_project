package dao

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go_project/ms_project/project_project/internal/data"
	"go_project/ms_project/project_project/internal/database/gorms"
	"gorm.io/gorm"
)

type MemberAccountDao struct {
	conn *gorms.GormConn
}

func (m *MemberAccountDao) FindByMemberId(ctx context.Context, memId int64) (ma *data.MemberAccount, err error) {
	session := m.conn.Session(ctx)
	err = session.Where("member_code=?", memId).Take(&ma).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return
}

func (m *MemberAccountDao) FindList(ctx context.Context, condition string,
	organizationCode int64, departmentCode int64,
	page int64, pageSize int64) (list []*data.MemberAccount, total int64, err error) {
	session := m.conn.Session(ctx)
	offset := (page - 1) * pageSize
	fmt.Println("FindList page:", page, "pageSize:", pageSize, "offset:", offset)
	err = session.Model(&data.MemberAccount{}).
		Where("organization_code=?", organizationCode).
		Where(condition).Limit(int(pageSize)).Offset(int(offset)).Find(&list).Error
	if err != nil {
		zap.L().Error("MemberAccountDao FindList error", zap.Error(err))
		return
	}
	err = session.Model(&data.MemberAccount{}).
		Where("organization_code=?", organizationCode).
		Where(condition).Count(&total).Error
	fmt.Println(list)
	return
}

func (m *MemberAccountDao) Save(ctx context.Context, ma *data.MemberAccount) error {
	return m.conn.Session(ctx).Create(ma).Error
}

func NewMemberAccountDao() *MemberAccountDao {
	return &MemberAccountDao{
		conn: gorms.New(),
	}
}
