chcp 65001
@echo off
color 0A
cls
echo,
echo 请选择要编译的系统环境：
echo,
echo 1. Windows_amd64
echo 2. linux_amd64

set /p action=请选择:
if %action% == 1 goto build_Windows_amd64
if %action% == 2 goto build_linux_amd64

:build_Windows_amd64
echo 编译Windows版本64位
SET CGO_ENABLED=0
SET GOOS=windows
SET GOARCH=amd64
go build -o project_user/target/project-user.exe project_user/main.go
go build -o project_api/target/project-api.exe project_api/main.go
pause
exit

:build_linux_amd64
echo 编译Linux版本64位
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build -o project_user/target/project-user project_user/main.go
go build -o project_api/target/project-api project_api/main.go
pause
exit