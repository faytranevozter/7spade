package room

import (
	"github.com/faytranevozter/7spade/services/ws/internal/apiclient"
	"github.com/faytranevozter/7spade/services/ws/internal/config"
	"github.com/faytranevozter/7spade/services/ws/internal/httpserver"
	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

type apiGameHistoryStore = apiclient.APIGameHistoryStore
type apiPlayerAccessChecker = apiclient.APIPlayerAccessChecker
type apiRoomMemberRemover = apiclient.APIRoomMemberRemover
type apiRoomReconciler = apiclient.APIRoomReconciler
type apiRoomSettingsStore = apiclient.APIRoomSettingsStore
type apiRoomStatusUpdater = apiclient.APIRoomStatusUpdater
type Config = config.Config

var LoadConfig = config.LoadConfig

type dependencyCheck = httpserver.DependencyCheck

var healthHandler = httpserver.HealthHandler
var postgresCheck = httpserver.PostgresCheck
var redisCheck = httpserver.RedisCheck
var withCORS = httpserver.WithCORS

type applicationControlsCache = apiclient.ApplicationControlsCache

var newApplicationControlsCache = apiclient.NewApplicationControlsCache

const applicationControlsRefreshInterval = apiclient.ApplicationControlsRefreshInterval

var parseToken = session.ParseToken
