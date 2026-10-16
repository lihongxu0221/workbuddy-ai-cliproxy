package growth

import (
	"context"
	"log"
	"time"
	"workbuddy-ai-cliproxy/internal/auth"
	"workbuddy-ai-cliproxy/internal/config"
)

// Scheduler 负责全自动化打卡与签到定时后台任务
type Scheduler struct {
	reporter *GlobalBonusReporter
	checkin  *CNCheckinExecutor
	cancel   context.CancelFunc
}

var (
	defaultScheduler *Scheduler
)

// StartScheduler 启动全局打卡调度协程
func StartScheduler() {
	if defaultScheduler != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		reporter: NewGlobalBonusReporter(),
		checkin:  NewCNCheckinExecutor(),
		cancel:   cancel,
	}
	defaultScheduler = s

	go s.runLoop(ctx)
}

// StopScheduler 优雅停止调度器
func StopScheduler() {
	if defaultScheduler != nil && defaultScheduler.cancel != nil {
		defaultScheduler.cancel()
		defaultScheduler = nil
	}
}

func (s *Scheduler) runLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// 启动时先行延迟 10 秒后执行一次初次扫描
	time.AfterFunc(10*time.Second, func() {
		s.checkAndTrigger(ctx, time.Now())
	})

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndTrigger(ctx, now)
		}
	}
}

func (s *Scheduler) checkAndTrigger(ctx context.Context, now time.Time) {
	cfg := config.Get()
	hour := now.Hour()

	// 国际区 4-Tick 打卡: 08:00, 12:00, 16:00, 20:00
	if cfg.DailyBonus && (hour == 8 || hour == 12 || hour == 16 || hour == 20) {
		s.RunGlobalReportAll(ctx)
	}

	// 国内区 2-Tick 签到: 09:00, 21:00
	if cfg.CheckinAuto && (hour == 9 || hour == 21) {
		s.RunCNCheckinAll(ctx)
	}
}

// RunGlobalReportAll 串行并带起搏间隔执行所有国际区账号打卡
func (s *Scheduler) RunGlobalReportAll(ctx context.Context) {
	store := auth.GetStore()
	accounts := store.ListByRealm(auth.RealmGlobal, false)
	if len(accounts) == 0 {
		return
	}

	for _, acc := range accounts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := s.reporter.ReportActivity(ctx, acc)
		if err != nil {
			log.Printf("[workbuddy] daily-report account %s failed: %v", acc.ID, err)
		} else {
			log.Printf("[workbuddy] daily-report account %s processed successfully", acc.ID)
		}

		// 账号之间强制休眠 1.5 秒起搏间隔，规避并发突发风控
		time.Sleep(1500 * time.Millisecond)
	}
}

// RunCNCheckinAll 串行执行所有国内区账号签到
func (s *Scheduler) RunCNCheckinAll(ctx context.Context) {
	store := auth.GetStore()
	accounts := store.ListByRealm(auth.RealmCN, false)
	if len(accounts) == 0 {
		return
	}

	for _, acc := range accounts {
		select {
		case <-ctx.Done():
			return
		default:
		}

		reward, err := s.checkin.Checkin(ctx, acc)
		if err != nil {
			log.Printf("[workbuddy] checkin account %s failed: %v", acc.ID, err)
		} else {
			log.Printf("[workbuddy] checkin account %s succeeded, reward credits: %d", acc.ID, reward)
		}

		time.Sleep(1500 * time.Millisecond)
	}
}
