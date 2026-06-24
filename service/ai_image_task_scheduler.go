package service

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

const aiImageTaskCheckCronSpec = "*/10 * * * *"

var (
	aiImageTaskCron *cron.Cron
	aiImageTaskOnce sync.Once
)

func StartAIImageTaskScheduler() {
	aiImageTaskOnce.Do(func() {
		aiImageTaskCron = cron.New()
		if _, err := aiImageTaskCron.AddFunc(aiImageTaskCheckCronSpec, CheckFrozenAIImageTasks); err != nil {
			log.Printf("add AI image task check cron failed cron=%s err=%v", aiImageTaskCheckCronSpec, err)
			return
		}
		aiImageTaskCron.Start()
	})
}
