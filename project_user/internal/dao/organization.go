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

func (o *OrganizationDao) FindOrganizationByMemId(ctx context.Context, memId int64) ([]*organization.Organization, error) {
	var orgs []*organization.Organization
	err := o.conn.Session(ctx).Where("member_id=?", memId).Find(&orgs).Error
	return orgs, err
}

func (o *OrganizationDao) FindOrganizationsByMemIds(ctx context.Context, memIds []int64) (map[int64][]*organization.Organization, error) {
	if len(memIds) == 0 {
		return map[int64][]*organization.Organization{}, nil
	}
	var orgs []*organization.Organization
	err := o.conn.Session(ctx).Where("member_id in (?)", memIds).Find(&orgs).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]*organization.Organization, len(memIds))
	for _, org := range orgs {
		result[org.MemberId] = append(result[org.MemberId], org)
	}
	return result, nil
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
