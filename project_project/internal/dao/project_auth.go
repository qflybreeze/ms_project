package dao

import (
	"context"
	"go_project/ms_project/project_project/internal/data"
	"go_project/ms_project/project_project/internal/database/gorms"
)

type ProjectAuthDao struct {
	conn *gorms.GormConn
}

func (p *ProjectAuthDao) FindAuthListPage(ctx context.Context, orgCode int64, page int64, pageSize int64) (list []*data.ProjectAuth, total int64, err error) {
	session := p.conn.Session(ctx)
	err = session.Model(&data.ProjectAuth{}).
		Where("organization_code=?", orgCode).
		Limit(int(pageSize)).
		Offset(int((page - 1) * pageSize)).
		Find(&list).Error
	err = session.Model(&data.ProjectAuth{}).
		Where("organization_code=?", orgCode).
		Count(&total).Error
	return
}

func (p *ProjectAuthDao) FindAuthList(ctx context.Context, orgCode int64) (list []*data.ProjectAuth, err error) {
	session := p.conn.Session(ctx)
	err = session.Model(&data.ProjectAuth{}).
		Where("organization_code=? and status=1", orgCode).Find(&list).Error
	return
}

// FindDefaultAuthByOrg 查找组织下 is_default=1 的角色（即"成员"角色）
func (p *ProjectAuthDao) FindDefaultAuthByOrg(ctx context.Context, orgCode int64) (*data.ProjectAuth, error) {
	var pa data.ProjectAuth
	err := p.conn.Session(ctx).
		Where("organization_code=? AND is_default=1 AND status=1", orgCode).
		Take(&pa).Error
	if err != nil {
		return nil, err
	}
	return &pa, nil
}

// SaveBatch 批量创建 auth 角色
func (p *ProjectAuthDao) SaveBatch(ctx context.Context, list []*data.ProjectAuth) error {
	return p.conn.Session(ctx).Create(&list).Error
}

func NewProjectAuthDao() *ProjectAuthDao {
	return &ProjectAuthDao{
		conn: gorms.New(),
	}
}
