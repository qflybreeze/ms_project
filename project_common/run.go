package common

import (
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(r *gin.Engine, server_name string, addr string, stop func()) {
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	//保证优雅启停
	go func() {
		log.Printf("%s is running on %s\n", server_name, addr)
		// 启动服务并判断是否正常关闭
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	quit := make(chan os.Signal)
	//SIGINT: 用户发送 Ctrl+C 信号触发kill -2
	//SIGTERM: 结束程序 kill -15
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("%s is shutting down\n", server_name)

	ctx, cancel_ := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel_()

	if stop != nil {
		stop()
	}
	//优雅关闭
	shutdownCh := make(chan error, 1)
	go func() {
		shutdownCh <- srv.Shutdown(ctx)
	}()

	select {
	//正常关闭或其他错误
	case err := <-shutdownCh:
		if err != nil {
			log.Printf("%s shutdown error: %v", server_name, err)
		} else {
			log.Printf("%s shutdown gracefully", server_name)
		}
	//超时关闭
	case <-ctx.Done():
		err := <-shutdownCh
		if err != nil {
			log.Printf("%s shutdown error: %v", server_name, err)
		}
		log.Printf("wait timeout")
	}
}
