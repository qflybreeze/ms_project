chcp 65001
cd project_user
docker build -t project-user:latest .
cd ..
docker-compose up -d