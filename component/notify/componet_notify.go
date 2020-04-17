package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/mochen302/agollo/component"
	"github.com/mochen302/agollo/env/config"

	"github.com/mochen302/agollo/component/log"
	"github.com/mochen302/agollo/env"
	"github.com/mochen302/agollo/protocol/http"
	"github.com/mochen302/agollo/storage"
	"github.com/mochen302/agollo/utils"
)

const (
	longPollInterval = 30 * time.Second //2s

	//notify timeout
	nofityConnectTimeout = 10 * time.Minute //10m

	//同步链接时间
	syncNofityConnectTimeout = 3 * time.Second //3s

	defaultNotificationId = int64(-1)
)

var (
	allNotifications *notificationsMap
)

type notification struct {
	NamespaceName  string `json:"namespaceName"`
	NotificationID int64  `json:"notificationId"`
}

// map[string]int64
type notificationsMap struct {
	notifications sync.Map
}

type apolloNotify struct {
	NotificationID int64  `json:"notificationId"`
	NamespaceName  string `json:"namespaceName"`
}

//InitAllNotifications 初始化notificationsMap
func InitAllNotifications(callback func(namespace string)) {
	appConfig := env.GetPlainAppConfig()
	ns := env.SplitNamespaces(appConfig.NamespaceName, callback)
	allNotifications = &notificationsMap{
		notifications: ns,
	}
}

func (n *notificationsMap) setNotify(namespaceName string, notificationID int64) {
	n.notifications.Store(namespaceName, notificationID)
}

func (n *notificationsMap) getNotify(namespace string) int64 {
	value, ok := n.notifications.Load(namespace)
	if !ok || value == nil {
		return 0
	}
	return value.(int64)
}

func (n *notificationsMap) GetNotifyLen() int {
	s := n.notifications
	l := 0
	s.Range(func(k, v interface{}) bool {
		l++
		return true
	})
	return l
}

func (n *notificationsMap) getNotifies(namespace string) string {
	notificationArr := make([]*notification, 0)
	if namespace == "" {
		n.notifications.Range(func(key, value interface{}) bool {
			namespaceName := key.(string)
			notificationID := value.(int64)
			notificationArr = append(notificationArr,
				&notification{
					NamespaceName:  namespaceName,
					NotificationID: notificationID,
				})
			return true
		})
	} else {
		notify, _ := n.notifications.LoadOrStore(namespace, defaultNotificationId)

		notificationArr = append(notificationArr,
			&notification{
				NamespaceName:  namespace,
				NotificationID: notify.(int64),
			})
	}

	j, err := json.Marshal(notificationArr)

	if err != nil {
		return ""
	}

	return string(j)
}

//ConfigComponent 配置组件
type ConfigComponent struct {
	LongPollInterval time.Duration
}

//Start 启动配置组件定时器
func (c *ConfigComponent) Start() {
	longPollIntervalNow := longPollInterval
	if c.LongPollInterval != 0 {
		longPollIntervalNow = c.LongPollInterval
	}

	t2 := time.NewTimer(longPollIntervalNow)
	//long poll for sync
	for {
		select {
		case <-t2.C:
			AsyncConfigs()
			t2.Reset(longPollIntervalNow)
		}
	}
}

//AsyncConfigs 异步同步所有配置文件中配置的namespace配置
func AsyncConfigs() error {
	return syncConfigs(utils.Empty, true, 0)
}

//SyncConfigs 同步同步所有配置文件中配置的namespace配置
func SyncConfigs() error {
	return SyncConfigsWithTimeout(syncNofityConnectTimeout)
}

//SyncConfigs 同步同步所有配置文件中配置的namespace配置
func SyncConfigsWithTimeout(syncTimeOut time.Duration) error {
	return syncConfigs(utils.Empty, false, syncTimeOut)
}

//SyncNamespaceConfig 同步同步一个指定的namespace配置
func SyncNamespaceConfig(namespace string, syncTimeOut time.Duration) error {
	return syncConfigs(namespace, false, syncTimeOut)
}

func syncConfigs(namespace string, isAsync bool, syncTimeOut time.Duration) error {

	remoteConfigs, err := notifyRemoteConfig(nil, namespace, isAsync, syncTimeOut)
	//if err != nil {
	//	appConfig := env.GetPlainAppConfig()
	//	loadBackupConfig(appConfig.NamespaceName, appConfig)
	//}

	if err != nil {
		return fmt.Errorf("notifySyncConfigServices: %s", err)
	}

	if len(remoteConfigs) == 0 {
		return nil
	}

	updateAllNotifications(remoteConfigs)

	//sync all config
	err = AutoSyncConfigServices(nil)

	if err != nil {
		if namespace != "" {
			return nil
		}
		//first sync fail then load config file
		appConfig := env.GetPlainAppConfig()
		loadBackupConfig(appConfig.NamespaceName, appConfig)
	}

	//sync all config
	return nil
}

func loadBackupConfig(namespace string, appConfig *config.AppConfig) {
	env.SplitNamespaces(namespace, func(namespace string) {
		config, _ := env.LoadConfigFile(appConfig.BackupConfigPath, namespace)
		if config != nil {
			storage.UpdateApolloConfig(config, false)
		}
	})
}

func toApolloConfig(resBody []byte) ([]*apolloNotify, error) {
	remoteConfig := make([]*apolloNotify, 0)

	err := json.Unmarshal(resBody, &remoteConfig)

	if err != nil {
		log.Error("Unmarshal Msg Fail,Error:", err)
		return nil, err
	}
	return remoteConfig, nil
}

func notifyRemoteConfig(newAppConfig *config.AppConfig, namespace string, isAsync bool, syncTimeOut time.Duration) ([]*apolloNotify, error) {
	appConfig := env.GetAppConfig(newAppConfig)
	if appConfig == nil {
		panic("can not find apollo config!please confirm!")
	}
	urlSuffix := getNotifyURLSuffix(allNotifications.getNotifies(namespace), appConfig, newAppConfig)

	//seelog.Debugf("allNotifications.getNotifies():%s",allNotifications.getNotifies())

	connectConfig := &env.ConnectConfig{
		URI: urlSuffix,
	}

	if !isAsync {
		if syncTimeOut == 0 {
			connectConfig.Timeout = syncNofityConnectTimeout
		} else {
			connectConfig.Timeout = syncTimeOut
		}

	} else {
		connectConfig.Timeout = nofityConnectTimeout
	}

	connectConfig.IsRetry = isAsync
	notifies, err := http.RequestRecovery(appConfig, connectConfig, &http.CallBack{
		SuccessCallBack: func(responseBody []byte) (interface{}, error) {
			return toApolloConfig(responseBody)
		},
		NotModifyCallBack: touchApolloConfigCache,
	})

	if notifies == nil {
		return nil, err
	}

	return notifies.([]*apolloNotify), err
}
func touchApolloConfigCache() interface{} {
	remoteConfig := make([]*apolloNotify, 0)
	return remoteConfig
}

func updateAllNotifications(remoteConfigs []*apolloNotify) {
	for _, remoteConfig := range remoteConfigs {
		if remoteConfig.NamespaceName == "" {
			continue
		}
		if allNotifications.getNotify(remoteConfig.NamespaceName) == 0 {
			continue
		}

		allNotifications.setNotify(remoteConfig.NamespaceName, remoteConfig.NotificationID)
	}
}

//AutoSyncConfigServicesSuccessCallBack 同步配置回调
func AutoSyncConfigServicesSuccessCallBack(responseBody []byte) (o interface{}, err error) {
	apolloConfig, err := env.CreateApolloConfigWithJSON(responseBody)

	if err != nil {
		log.Error("Unmarshal Msg Fail,Error:", err)
		return nil, err
	}
	appConfig := env.GetPlainAppConfig()

	storage.UpdateApolloConfig(apolloConfig, appConfig.GetIsBackupConfig())

	return nil, nil
}

//AutoSyncConfigServices 自动同步配置
func AutoSyncConfigServices(newAppConfig *config.AppConfig) error {
	return autoSyncNamespaceConfigServices(newAppConfig, allNotifications, syncNofityConnectTimeout)
}

//AutoSyncConfigServices 自动同步配置
func AutoSyncConfigServicesWithTimtout(newAppConfig *config.AppConfig, syncTimeOut time.Duration) error {
	return autoSyncNamespaceConfigServices(newAppConfig, allNotifications, syncTimeOut)
}

func autoSyncNamespaceConfigServices(newAppConfig *config.AppConfig, allNotifications *notificationsMap, syncTimeOut time.Duration) error {
	appConfig := env.GetAppConfig(newAppConfig)
	if appConfig == nil {
		panic("can not find apollo config!please confirm!")
	}

	var err error
	allNotifications.notifications.Range(func(key, value interface{}) bool {
		namespace := key.(string)
		urlSuffix := component.GetConfigURLSuffix(appConfig, namespace)

		_, err = http.RequestRecovery(appConfig, &env.ConnectConfig{
			URI:     urlSuffix,
			Timeout: syncTimeOut,
		}, &http.CallBack{
			SuccessCallBack:   AutoSyncConfigServicesSuccessCallBack,
			NotModifyCallBack: touchApolloConfigCache,
		})
		if err != nil {
			return false
		}
		return true
	})
	return err
}

func getNotifyURLSuffix(notifications string, config *config.AppConfig, newConfig *config.AppConfig) string {
	c := config
	if newConfig != nil {
		c = newConfig
	}
	return fmt.Sprintf("notifications/v2?appId=%s&cluster=%s&notifications=%s",
		url.QueryEscape(c.AppID),
		url.QueryEscape(c.Cluster),
		url.QueryEscape(notifications))
}
