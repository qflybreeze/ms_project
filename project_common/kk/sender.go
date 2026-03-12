package kk

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// TODO实现优雅启停
const retries = 3

type LogData struct {
	Topic string
	Data  []byte //传入json
}

type KafkaWriter struct {
	w    *kafka.Writer
	Data chan LogData
}

func (w *KafkaWriter) Close() {
	if w.w != nil {
		w.w.Close()
	}
}

func GetWriter(addr string) *KafkaWriter {
	w := &kafka.Writer{
		Addr:     kafka.TCP(addr),
		Balancer: &kafka.LeastBytes{}, // 最少字节数的分区
	}
	k := &KafkaWriter{w: w, Data: make(chan LogData, 100)}
	go k.sendMsg()
	return k
}

// TODO单进程挂了处理？增强健壮性
func (kw *KafkaWriter) Send(data LogData) {
	select {
	case kw.Data <- data:
	default:
		// channel 满时直接丢弃，避免阻塞业务请求
	}
}

func (kw *KafkaWriter) sendMsg() {
	for data := range kw.Data {
		msg := []kafka.Message{
			{
				Topic: data.Topic,
				Key:   []byte("logMsg"),
				Value: data.Data,
			},
		}
		var err error
		for i := 0; i < retries; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err = kw.w.WriteMessages(ctx, msg...)
			cancel() // 每次循环及时释放 context，避免泄漏
			if err == nil {
				break
			}
			if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
				time.Sleep(time.Millisecond * 250 * time.Duration(i+1)) // 指数退避
				continue
			}
			log.Println("kafka send log writer msg err", err.Error())
			break // 非重试类错误直接退出
		}
	}
}
