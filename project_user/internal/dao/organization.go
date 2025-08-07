package dao

import (
	"context"
	"go_project/ms_project/project_user/internal/data/organization"
	"go_project/ms_project/project_user/internal/database"
	"go_project/ms_project/project_user/internal/database/gorms"
)

type OrganizationDao struct {
	conn *gorms.GormConn
}

func NewOrganizationDao() *OrganizationDao {
	return &OrganizationDao{
		conn: gorms.New(),
	}
}

func (o *OrganizationDao) SaveOrganization(conn database.DbConn, ctx context.Context, org *organization.Organization) error {
	o.conn = conn.(*gorms.GormConn)
	return o.conn.Tx(ctx).Create(org).Error
}
