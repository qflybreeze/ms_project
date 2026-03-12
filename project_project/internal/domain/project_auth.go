package domain

import (
	"context"
	"go_project/ms_project/project_common/errs"
	"go_project/ms_project/project_project/internal/dao"
	"go_project/ms_project/project_project/internal/data"
	"go_project/ms_project/project_project/internal/database/gorms"
	"go_project/ms_project/project_project/internal/repo"
	"go_project/ms_project/project_project/pkg/model"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type ProjectAuthDomain struct {
	projectAuthRepo       repo.ProjectAuthRepo
	UserRpcDomain         *UserRpcDomain
	projectNodeDomain     *ProjectNodeDomain
	ProjectAuthNodeDomain *ProjectAuthNodeDomain
	accountDomain         *AccountDomain
}

func (d *ProjectAuthDomain) AuthList(orgCode int64) ([]*data.ProjectAuthDisplay, *errs.BError) {
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := d.projectAuthRepo.FindAuthList(c, orgCode)
	if err != nil {
		zap.L().Error("project AuthList projectAuthRepo.FindAuthList error", zap.Error(err))
		return nil, model.DBError
	}
	var pdList []*data.ProjectAuthDisplay
	for _, v := range list {
		display := v.ToDisplay()
		pdList = append(pdList, display)
	}
	return pdList, nil
}

func (d *ProjectAuthDomain) AuthListPage(orgCode int64, page int64, pageSize int64) ([]*data.ProjectAuthDisplay, int64, *errs.BError) {
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, total, err := d.projectAuthRepo.FindAuthListPage(c, orgCode, page, pageSize)
	if err != nil {
		zap.L().Error("project AuthList projectAuthRepo.FindAuthList error", zap.Error(err))
		return nil, 0, model.DBError
	}
	var pdList []*data.ProjectAuthDisplay
	for _, v := range list {
		display := v.ToDisplay()
		pdList = append(pdList, display)
	}
	return pdList, total, nil
}

func (d *ProjectAuthDomain) AllNodeAndAuth(authId int64) ([]*data.ProjectNodeAuthTree, []string, *errs.BError) {
	treeList, err := d.projectNodeDomain.NodeList()
	if err != nil {
		return nil, nil, err
	}
	checkedList, err := d.ProjectAuthNodeDomain.AuthNodeList(authId)
	if err != nil {
		return nil, nil, err
	}
	list := data.ToAuthNodeTreeList(treeList, checkedList)
	return list, checkedList, nil
}

func (d *ProjectAuthDomain) AuthNodes(memberId int64) ([]string, *errs.BError) {
	account, err := d.accountDomain.FindAccount(memberId)
	if err != nil {
		return nil, err
	}
	if account == nil {
		// 新注册用户首次访问时自动创建默认 account 和组织权限角色
		zap.L().Info("用户无 account 记录，执行初始化", zap.Int64("memberId", memberId))
		account, err = d.initDefaultAccount(memberId)
		if err != nil {
			return nil, err
		}
	}
	authorize := account.Authorize
	authId, _ := strconv.ParseInt(authorize, 10, 64)
	authNodeList, dbErr := d.ProjectAuthNodeDomain.AuthNodeList(authId)
	if dbErr != nil {
		return nil, model.DBError
	}
	return authNodeList, nil
}

// 为新用户自动创建组织权限角色和 account 记录
func (d *ProjectAuthDomain) initDefaultAccount(memberId int64) (*data.MemberAccount, *errs.BError) {
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 通过 RPC 获取用户信息
	memberInfo, rpcErr := d.UserRpcDomain.MemberInfo(c, memberId)
	if rpcErr != nil {
		zap.L().Error("initDefaultAccount 获取用户信息失败", zap.Error(rpcErr))
		return nil, model.DBError
	}

	// 获取用户的组织列表，取个人组织
	orgList, rpcErr := d.UserRpcDomain.MyOrgList(c, memberId)
	if rpcErr != nil || orgList == nil || len(orgList.OrganizationList) == 0 {
		zap.L().Error("initDefaultAccount 获取用户组织失败", zap.Error(rpcErr))
		return nil, model.DBError
	}
	orgCode := orgList.OrganizationList[0].Id

	// 检查该组织是否已有权限角色，没有则创建
	adminAuthId, err := d.ensureOrgAuthRoles(c, orgCode)
	if err != nil {
		return nil, err
	}

	// 创建 member_account 记录
	now := time.Now().UnixMilli()
	account := &data.MemberAccount{
		MemberCode:       memberId,
		OrganizationCode: orgCode,
		Authorize:        strconv.FormatInt(adminAuthId, 10),
		IsOwner:          1,
		Name:             memberInfo.Name,
		Mobile:           memberInfo.Mobile,
		Email:            memberInfo.Email,
		CreateTime:       now,
		LastLoginTime:    now,
		Status:           1,
	}
	if saveErr := d.accountDomain.accountRepo.Save(c, account); saveErr != nil {
		zap.L().Error("initDefaultAccount 创建 account 失败", zap.Error(saveErr))
		return nil, model.DBError
	}
	zap.L().Info("初始化完成：已为用户创建默认 account",
		zap.Int64("memberId", memberId),
		zap.Int64("orgCode", orgCode),
		zap.Int64("adminAuthId", adminAuthId))

	return account, nil
}

// 确保组织下存在管理员和成员角色，返回管理员角色 ID
func (d *ProjectAuthDomain) ensureOrgAuthRoles(ctx context.Context, orgCode int64) (int64, *errs.BError) {
	// 先查是否已有该组织的角色
	list, dbErr := d.projectAuthRepo.FindAuthList(ctx, orgCode)
	if dbErr != nil {
		return 0, model.DBError
	}
	// 已有角色，找管理员角色返回
	if len(list) > 0 {
		for _, a := range list {
			if a.Type == "admin" {
				return a.Id, nil
			}
		}
		// 有角色但没 admin，返回第一个
		return list[0].Id, nil
	}

	// 没有角色，创建管理员 + 成员
	now := time.Now().UnixMilli()
	adminAuth := &data.ProjectAuth{
		OrganizationCode: orgCode,
		Title:            "管理员",
		Status:           1,
		Desc:             "管理员",
		CreateAt:         now,
		IsDefault:        0,
		Type:             "admin",
	}
	memberAuth := &data.ProjectAuth{
		OrganizationCode: orgCode,
		Title:            "成员",
		Status:           1,
		Desc:             "成员",
		CreateAt:         now,
		IsDefault:        1,
		Type:             "member",
	}
	if err := d.projectAuthRepo.SaveBatch(ctx, []*data.ProjectAuth{adminAuth, memberAuth}); err != nil {
		zap.L().Error("创建组织权限角色失败", zap.Error(err))
		return 0, model.DBError
	}

	d.copyGlobalAuthNodes(ctx, 1, adminAuth.Id)
	d.copyGlobalAuthNodes(ctx, 2, memberAuth.Id)

	zap.L().Info("已为组织创建默认权限角色",
		zap.Int64("orgCode", orgCode),
		zap.Int64("adminAuthId", adminAuth.Id),
		zap.Int64("memberAuthId", memberAuth.Id))

	return adminAuth.Id, nil
}

// 将全局角色的权限节点复制到新角色
func (d *ProjectAuthDomain) copyGlobalAuthNodes(ctx context.Context, globalAuthId, newAuthId int64) {
	nodes, err := d.ProjectAuthNodeDomain.AuthNodeList(globalAuthId)
	if err != nil || len(nodes) == 0 {
		return
	}
	conn := gorms.NewTran()
	if saveErr := dao.NewProjectAuthNodeDao().Save(ctx, conn, newAuthId, nodes); saveErr != nil {
		zap.L().Warn("复制权限节点失败", zap.Int64("from", globalAuthId), zap.Int64("to", newAuthId), zap.Error(saveErr))
	}
}

//func (d *ProjectAuthDomain) Save(conn database.DbConn, authId int64, nodes []string) *errs.BError {
//	err := d.ProjectAuthNodeDomain.Save(context.Background(), conn, authId, nodes)
//	if err != nil {
//		return err
//	}
//	return nil
//}

func NewProjectAuthDomain() *ProjectAuthDomain {
	return &ProjectAuthDomain{
		projectAuthRepo:       dao.NewProjectAuthDao(),
		UserRpcDomain:         NewUserRpcDomain(),
		projectNodeDomain:     NewProjectNodeDomain(),
		ProjectAuthNodeDomain: NewProjectAuthNodeDomain(),
		accountDomain:         NewAccountDomain(),
	}
}
