package component

import (
	"fmt"
	"net/url"

	"github.com/mochen302/agollo/env/config"

	"github.com/mochen302/agollo/env"
	"github.com/mochen302/agollo/utils"
)

//AbsComponent 定时组件
type AbsComponent interface {
	Start()
}

//StartRefreshConfig 开始定时服务
func StartRefreshConfig(component AbsComponent) {
	component.Start()
}

//GetConfigURLSuffix 获取apollo config server的路径
func GetConfigURLSuffix(config *config.AppConfig, namespaceName string) string {
	if config == nil {
		return ""
	}
	return fmt.Sprintf("configs/%s/%s/%s?releaseKey=%s&ip=%s",
		url.QueryEscape(config.AppID),
		url.QueryEscape(config.Cluster),
		url.QueryEscape(namespaceName),
		url.QueryEscape(env.GetCurrentApolloConfigReleaseKey(namespaceName)),
		utils.GetInternal())
}
