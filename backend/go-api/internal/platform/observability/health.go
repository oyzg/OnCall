package observability

import (
	"context"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/cache"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/grpcclient"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
)

type ComponentStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CheckedAt string `json:"checked_at"`
}

type HealthReport struct {
	Service    string            `json:"service"`
	Env        string            `json:"env"`
	Status     string            `json:"status"`
	Components []ComponentStatus `json:"components"`
}

func BuildHealthReport(cfg config.Config) HealthReport {
	components := []ComponentStatus{
		check("mysql", db.Probe{DSN: cfg.MySQL.DSN, Timeout: cfg.MySQL.PingTimeout}.Ping),
		check("redis", cache.Probe{Addr: cfg.Redis.Addr, Timeout: cfg.Redis.PingTimeout}.Ping),
		check("python_ai_grpc", grpcclient.Probe{Target: cfg.AI.GRPCTarget, Timeout: cfg.AI.PingTimeout}.Ping),
	}

	report := HealthReport{
		Service:    cfg.App.Name,
		Env:        cfg.App.Env,
		Status:     "ok",
		Components: components,
	}

	for _, component := range components {
		if component.Status != "up" {
			report.Status = "degraded"
			break
		}
	}

	return report
}

func check(name string, ping func(context.Context) error) ComponentStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	status := ComponentStatus{
		Name:      name,
		Status:    "up",
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	if err := ping(ctx); err != nil {
		status.Status = "down"
		status.Error = err.Error()
	}

	return status
}
