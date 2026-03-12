package domain

import (
	"context"
	"go_project/ms_project/project_common/errs"
	"go_project/ms_project/project_common/kk"
	"go_project/ms_project/project_project/config"
	"go_project/ms_project/project_project/internal/dao"
	"go_project/ms_project/project_project/internal/repo"
	"go_project/ms_project/project_project/pkg/model"
)

type TaskDomain struct {
	taskRepo repo.TaskRepo
}

func NewTaskDomain() *TaskDomain {
	return &TaskDomain{
		taskRepo: dao.NewTaskDao(),
	}
}

func (d *TaskDomain) FindProjectIdByTaskId(taskId int64) (int64, bool, *errs.BError) {
	config.SendLog(kk.Info("query", "TaskDomain.FindProjectIdByTaskId", kk.FieldMap{
		"taskId": taskId,
	}))
	task, err := d.taskRepo.FindTaskById(context.Background(), taskId)
	if err != nil {
		config.SendLog(kk.Error(err, "TaskDomain.FindProjectIdByTaskId.taskRepo.FindTaskById", kk.FieldMap{
			"action": "query",
			"taskId": taskId,
		}))
		return 0, false, model.DBError
	}
	if task == nil {
		return 0, false, nil
	}
	return task.ProjectCode, true, nil
}
