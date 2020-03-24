package agollo

import (
	"github.com/mochen302/agollo/agcache"
	"github.com/mochen302/agollo/component"
	"github.com/mochen302/agollo/component/log"
	"github.com/mochen302/agollo/component/notify"
	"github.com/mochen302/agollo/component/serverlist"
	"github.com/mochen302/agollo/env"
	"github.com/mochen302/agollo/env/config"
	"github.com/mochen302/agollo/loadbalance/roundrobin"
	"github.com/mochen302/agollo/storage"
)

var (
	initAppConfigFunc func() (*config.AppConfig, error)
)

func init() {
	roundrobin.InitLoadBalance()
}

//InitCustomConfig init config by custom
func InitCustomConfig(loadAppConfig func() (*config.AppConfig, error)) {
	initAppConfigFunc = loadAppConfig
}

//start apollo
func Start() error {
	return startAgollo()
}

//SetLogger 设置自定义logger组件
func SetLogger(loggerInterface log.LoggerInterface) {
	if loggerInterface != nil {
		log.InitLogger(loggerInterface)
	}
}

//SetCache 设置自定义cache组件
func SetCache(cacheFactory agcache.CacheFactory) {
	if cacheFactory != nil {
		agcache.UseCacheFactory(cacheFactory)
		storage.InitConfigCache()
	}
}

func startAgollo() error {
	// 有了配置之后才能进行初始化
	if err := env.InitConfig(initAppConfigFunc); err != nil {
		return err
	}
	notify.InitAllNotifications(nil)
	serverlist.InitSyncServerIPList()

	//first sync
	if err := notify.SyncConfigs(); err != nil {
		return err
	}
	log.Debug("init notifySyncConfigServices finished")

	//start long poll sync config
	go component.StartRefreshConfig(&notify.ConfigComponent{})

	log.Info("agollo start finished ! ")

	return nil
}
