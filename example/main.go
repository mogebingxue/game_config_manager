package main

import (
	"github.com/mogebingxue/game_config_manager"
	"github.com/mogebingxue/game_config_manager/example/conf_go/testpkg"
	"log/slog"
	"time"
)

func main() {
	cfg, err := config.LoadConfig("./conf.yaml")
	if err != nil {
		return
	}
	config.GetConfigManager().StartService(cfg.DataPath)
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			slog.Info("table", "table", testpkg.GetTestTable())
		}
	}
}
