package server

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"sub2api-wails/ent"
	"sub2api-wails/internal/config"
	"sub2api-wails/internal/handler"
	"sub2api-wails/internal/handler/admin"
	"sub2api-wails/internal/payment"
	"sub2api-wails/internal/pkg/antigravity"
	"sub2api-wails/internal/pkg/redismem"
	"sub2api-wails/internal/pkg/websearch"
	"sub2api-wails/internal/repository"
	"sub2api-wails/internal/server/middleware"
	"sub2api-wails/internal/service"

	"github.com/gin-gonic/gin"
)

type noopRedeemCache struct{}

func (noopRedeemCache) GetRedeemAttemptCount(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (noopRedeemCache) IncrementRedeemAttemptCount(_ context.Context, _ int64) error {
	return nil
}
func (noopRedeemCache) AcquireRedeemLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (noopRedeemCache) ReleaseRedeemLock(_ context.Context, _ string) error {
	return nil
}

func Initialize(r *gin.Engine, cfg *config.Config, entClient *ent.Client, db *sql.DB) (*http.Server, error) {
	redisStub := redismem.NewRedisStub()

	gatewayCache := repository.NewGatewayCache(redisStub)
	billingCache := repository.NewBillingCache(redisStub)
	apiKeyCache := repository.NewAPIKeyCache(redisStub)
	tempUnschedCache := repository.NewTempUnschedCache(redisStub)
	timeoutCounterCache := repository.NewTimeoutCounterCache(redisStub)
	openAI403CounterCache := repository.NewOpenAI403CounterCache(redisStub)
	internal500CounterCache := repository.NewInternal500CounterCache(redisStub)
	concurrencyCache := repository.ProvideConcurrencyCache(redisStub, cfg)
	sessionLimitCache := repository.ProvideSessionLimitCache(redisStub, cfg)
	rpmCache := repository.NewRPMCache(redisStub)
	userRPMCache := repository.NewUserRPMCache(redisStub)
	userMsgQueueCache := repository.NewUserMsgQueueCache(redisStub)
	dashboardCache := repository.NewDashboardCache(redisStub, cfg)
	emailCache := repository.NewEmailCache(redisStub)
	identityCache := repository.NewIdentityCache(redisStub)
	updateCache := repository.NewUpdateCache(redisStub)
	geminiTokenCache := repository.NewGeminiTokenCache(redisStub)
	schedulerCache := repository.ProvideSchedulerCache(redisStub, cfg)
	proxyLatencyCache := repository.NewProxyLatencyCache(redisStub)
	totpCache := repository.NewTotpCache(redisStub)
	refreshTokenCache := repository.NewRefreshTokenCache(redisStub)
	errorPassthroughCache := repository.NewErrorPassthroughCache(redisStub)
	tlsFPProfileCache := repository.NewTLSFingerprintProfileCache(redisStub)
	contentModerationHashCache := repository.NewContentModerationHashCache(redisStub)

	userRepo := repository.NewUserRepository(entClient, db)
	apiKeyRepo := repository.NewAPIKeyRepository(entClient, db)
	groupRepo := repository.NewGroupRepository(entClient, db)
	accountRepo := repository.NewAccountRepository(entClient, db, schedulerCache)
	scheduledTestPlanRepo := repository.NewScheduledTestPlanRepository(db)
	scheduledTestResultRepo := repository.NewScheduledTestResultRepository(db)
	proxyRepo := repository.NewProxyRepository(entClient, db)
	redeemCodeRepo := repository.NewRedeemCodeRepository(entClient)
	promoCodeRepo := repository.NewPromoCodeRepository(entClient)
	announcementRepo := repository.NewAnnouncementRepository(entClient)
	announcementReadRepo := repository.NewAnnouncementReadRepository(entClient)
	usageLogRepo := repository.NewUsageLogRepository(entClient, db)
	usageBillingRepo := repository.NewUsageBillingRepository(entClient, db)
	idempotencyRepo := repository.NewIdempotencyRepository(entClient, db)
	usageCleanupRepo := repository.NewUsageCleanupRepository(entClient, db)
	dashboardAggregationRepo := repository.NewDashboardAggregationRepository(db)
	settingRepo := repository.NewSettingRepository(entClient)
	opsRepo := repository.NewOpsRepository(db)
	userSubRepo := repository.NewUserSubscriptionRepository(entClient)
	userAttrDefRepo := repository.NewUserAttributeDefinitionRepository(entClient)
	userAttrValueRepo := repository.NewUserAttributeValueRepository(entClient)
	userGroupRateRepo := repository.NewUserGroupRateRepository(db)
	errorPassthroughRepo := repository.NewErrorPassthroughRepository(entClient)
	tlsFPProfileRepo := repository.NewTLSFingerprintProfileRepository(entClient)
	channelRepo := repository.NewChannelRepository(db)
	channelMonitorRepo := repository.NewChannelMonitorRepository(entClient, db)
	channelMonitorTemplateRepo := repository.NewChannelMonitorRequestTemplateRepository(entClient, db)
	contentModerationRepo := repository.NewContentModerationRepository(db)
	affiliateRepo := repository.NewAffiliateRepository(entClient, db)
	schedulerOutboxRepo := repository.NewSchedulerOutboxRepository(db)

	aesEncryptor, err := repository.NewAESEncryptor(cfg)
	if err != nil {
		return nil, err
	}
	pgDumper := repository.NewPgDumper(cfg)
	s3BackupStoreFactory := repository.NewS3BackupStoreFactory()
	httpUpstream := repository.NewHTTPUpstream(cfg)
	claudeUsageFetcher := repository.NewClaudeUsageFetcher(httpUpstream)
	claudeOAuthClient := repository.NewClaudeOAuthClient()
	openAIOAuthClient := repository.NewOpenAIOAuthClient()
	geminiOAuthClient := repository.NewGeminiOAuthClient(cfg)
	geminiCliCodeAssistClient := repository.NewGeminiCliCodeAssistClient()
	geminiDriveClient := repository.NewGeminiDriveClient()
	turnstileVerifier := repository.NewTurnstileVerifier()
	pricingRemoteClient := repository.ProvidePricingRemoteClient(cfg)
	githubReleaseClient := repository.ProvideGitHubReleaseClient(cfg)
	proxyExitInfoProber := repository.NewProxyExitInfoProber(cfg)

	timingWheelSvc, err := service.ProvideTimingWheelService()
	if err != nil {
		return nil, err
	}
	digestStore := service.NewDigestSessionStore()
	usageCache := service.NewUsageCache()
	dataManagementSvc := service.NewDataManagementService()

	pricingSvc, err := service.ProvidePricingService(cfg, pricingRemoteClient)
	if err != nil {
		return nil, err
	}
	billingSvc := service.NewBillingService(cfg, pricingSvc)

	settingSvc := service.ProvideSettingService(settingRepo, groupRepo, proxyRepo, cfg)

	emailSvc := service.NewEmailService(settingRepo, emailCache)
	notificationEmailSvc := service.NewNotificationEmailService(settingRepo, emailSvc)
	emailQueueSvc := service.ProvideEmailQueueService(emailSvc)
	turnstileSvc := service.NewTurnstileService(settingSvc, turnstileVerifier)
	identitySvc := service.NewIdentityService(identityCache)

	oauthSvc := service.NewOAuthService(proxyRepo, claudeOAuthClient)
	openAIOAuthSvc := service.NewOpenAIOAuthService(proxyRepo, openAIOAuthClient)
	geminiOAuthSvc := service.NewGeminiOAuthService(proxyRepo, geminiOAuthClient, geminiCliCodeAssistClient, geminiDriveClient, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(proxyRepo)

	geminiQuotaSvc := service.NewGeminiQuotaService(cfg, settingRepo)
	tokenCacheInvalidator := service.NewCompositeTokenCacheInvalidator(geminiTokenCache)
	oauthRefreshAPI := service.ProvideOAuthRefreshAPI(accountRepo, geminiTokenCache)

	claudeTokenProvider := service.ProvideClaudeTokenProvider(accountRepo, geminiTokenCache, oauthSvc, oauthRefreshAPI)
	openAITokenProvider := service.ProvideOpenAITokenProvider(accountRepo, geminiTokenCache, openAIOAuthSvc, oauthRefreshAPI)
	geminiTokenProvider := service.ProvideGeminiTokenProvider(accountRepo, geminiTokenCache, geminiOAuthSvc, oauthRefreshAPI)
	antigravityTokenProvider := service.ProvideAntigravityTokenProvider(accountRepo, geminiTokenCache, antigravityOAuthSvc, oauthRefreshAPI, tempUnschedCache)

	concurrencySvc := service.ProvideConcurrencyService(concurrencyCache, accountRepo, cfg)
	billingCacheSvc := service.ProvideBillingCacheService(billingCache, userRepo, userSubRepo, apiKeyRepo, userRPMCache, userGroupRateRepo, cfg)
	apiKeySvc := service.ProvideAPIKeyService(apiKeyRepo, userRepo, groupRepo, userSubRepo, userGroupRateRepo, apiKeyCache, cfg, billingCacheSvc)
	authCacheInvalidator := service.ProvideAPIKeyAuthCacheInvalidator(apiKeySvc)

	userSvc := service.NewUserService(userRepo, settingRepo, authCacheInvalidator, billingCache)
	_ = service.NewGroupService(groupRepo, authCacheInvalidator)
	_ = service.NewAccountService(accountRepo, groupRepo)
	_ = service.NewProxyService(proxyRepo)

	dashboardAggSvc := service.ProvideDashboardAggregationService(dashboardAggregationRepo, timingWheelSvc, cfg)
	dashboardSvc := service.NewDashboardService(usageLogRepo, dashboardAggregationRepo, dashboardCache, cfg)
	usageSvc := service.NewUsageService(usageLogRepo, userRepo, entClient, authCacheInvalidator)

	rateLimitSvc := service.ProvideRateLimitService(accountRepo, usageLogRepo, cfg, geminiQuotaSvc, tempUnschedCache, timeoutCounterCache, openAI403CounterCache, settingSvc, tokenCacheInvalidator)

	schedulerSnapshotSvc := service.ProvideSchedulerSnapshotService(schedulerCache, schedulerOutboxRepo, accountRepo, groupRepo, cfg)
	deferredSvc := service.ProvideDeferredService(accountRepo, timingWheelSvc)
	accountExpirySvc := service.ProvideAccountExpiryService(accountRepo)
	subscriptionExpirySvc := service.ProvideSubscriptionExpiryService(userSubRepo, settingRepo, notificationEmailSvc)

	subscriptionSvc := service.NewSubscriptionService(groupRepo, userSubRepo, billingCacheSvc, entClient, cfg)

	var redeemCache service.RedeemCache = noopRedeemCache{}

	affiliateSvc := service.NewAffiliateService(affiliateRepo, settingSvc, authCacheInvalidator, billingCacheSvc)

	redeemSvc := service.NewRedeemService(redeemCodeRepo, userRepo, subscriptionSvc, redeemCache, billingCacheSvc, entClient, authCacheInvalidator, affiliateSvc)
	promoSvc := service.NewPromoService(promoCodeRepo, userRepo, billingCacheSvc, entClient, authCacheInvalidator)
	announcementSvc := service.NewAnnouncementService(announcementRepo, announcementReadRepo, userRepo, userSubRepo)

	tlsFPProfileSvc := service.NewTLSFingerprintProfileService(tlsFPProfileRepo, tlsFPProfileCache)
	errorPassthroughSvc := service.NewErrorPassthroughService(errorPassthroughRepo, errorPassthroughCache)
	contentModerationSvc := service.NewContentModerationService(settingRepo, contentModerationRepo, contentModerationHashCache, groupRepo, userRepo, authCacheInvalidator, emailSvc)

	antigravityQuotaFetcher := service.NewAntigravityQuotaFetcher(proxyRepo)
	accountUsageSvc := service.NewAccountUsageService(accountRepo, usageLogRepo, claudeUsageFetcher, geminiQuotaSvc, antigravityQuotaFetcher, usageCache, identityCache, tlsFPProfileSvc)

	channelSvc := service.NewChannelService(channelRepo, groupRepo, authCacheInvalidator, pricingSvc)
	modelPricingResolver := service.NewModelPricingResolver(channelSvc, billingSvc)
	balanceNotifySvc := service.ProvideBalanceNotifyService(emailSvc, settingRepo, accountRepo, notificationEmailSvc)

	gatewaySvc := service.NewGatewayService(
		accountRepo, groupRepo, usageLogRepo, usageBillingRepo,
		userRepo, userSubRepo, userGroupRateRepo,
		gatewayCache, cfg, schedulerSnapshotSvc, concurrencySvc,
		billingSvc, rateLimitSvc, billingCacheSvc, identitySvc,
		httpUpstream, deferredSvc, claudeTokenProvider,
		sessionLimitCache, rpmCache, digestStore, settingSvc,
		tlsFPProfileSvc, channelSvc, modelPricingResolver, balanceNotifySvc,
	)

	openAIGatewaySvc := service.NewOpenAIGatewayService(
		accountRepo, usageLogRepo, usageBillingRepo,
		userRepo, userSubRepo, userGroupRateRepo,
		gatewayCache, cfg, schedulerSnapshotSvc, concurrencySvc,
		billingSvc, rateLimitSvc, billingCacheSvc,
		httpUpstream, deferredSvc, openAITokenProvider,
		modelPricingResolver, channelSvc, balanceNotifySvc, settingSvc,
	)

	antigravityGatewaySvc := service.NewAntigravityGatewayService(
		accountRepo, gatewayCache, schedulerSnapshotSvc,
		antigravityTokenProvider, rateLimitSvc, httpUpstream,
		settingSvc, internal500CounterCache,
	)

	geminiCompatSvc := service.NewGeminiMessagesCompatService(
		accountRepo, groupRepo, gatewayCache, schedulerSnapshotSvc,
		geminiTokenProvider, rateLimitSvc, httpUpstream,
		antigravityGatewaySvc, cfg,
	)

	accountTestSvc := service.NewAccountTestService(
		accountRepo, geminiTokenProvider, claudeTokenProvider,
		antigravityGatewaySvc, httpUpstream, cfg, tlsFPProfileSvc,
	)

	crsSyncSvc := service.NewCRSSyncService(accountRepo, proxyRepo, oauthSvc, openAIOAuthSvc, geminiOAuthSvc, cfg)

	userAttrSvc := service.NewUserAttributeService(userAttrDefRepo, userAttrValueRepo)
	totpSvc := service.NewTotpService(userRepo, aesEncryptor, totpCache, settingSvc, emailSvc, emailQueueSvc)

	userMsgQueueSvc := service.ProvideUserMessageQueueService(userMsgQueueCache, rpmCache, cfg)
	usageRecordWorkerPool := service.NewUsageRecordWorkerPool(cfg)
	usageCleanupSvc := service.ProvideUsageCleanupService(usageCleanupRepo, timingWheelSvc, dashboardAggSvc, cfg)

	idempotencyCoordinator := service.ProvideIdempotencyCoordinator(idempotencyRepo, cfg)
	systemOpLockSvc := service.ProvideSystemOperationLockService(idempotencyRepo, cfg)
	idempotencyCleanupSvc := service.ProvideIdempotencyCleanupService(idempotencyRepo, cfg)

	scheduledTestSvc := service.ProvideScheduledTestService(scheduledTestPlanRepo, scheduledTestResultRepo)
	scheduledTestRunnerSvc := service.ProvideScheduledTestRunnerService(scheduledTestPlanRepo, scheduledTestSvc, accountTestSvc, rateLimitSvc, cfg)

	groupCapacitySvc := service.NewGroupCapacityService(accountRepo, groupRepo, concurrencySvc, sessionLimitCache, rpmCache)

	encKey, _ := payment.ProvideEncryptionKey(cfg)
	paymentRegistry := payment.NewRegistry()
	paymentLoadBalancer := payment.NewDefaultLoadBalancer(entClient, []byte(encKey))
	paymentConfigSvc := service.NewPaymentConfigService(entClient, settingRepo, []byte(encKey))
	paymentSvc := service.NewPaymentService(entClient, paymentRegistry, paymentLoadBalancer, redeemSvc, subscriptionSvc, paymentConfigSvc, userRepo, groupRepo, affiliateSvc)
	paymentSvc.SetNotificationEmailService(notificationEmailSvc)
	paymentOrderExpirySvc := service.ProvidePaymentOrderExpiryService(paymentSvc)

	channelMonitorSvc := service.ProvideChannelMonitorService(channelMonitorRepo, aesEncryptor)
	channelMonitorRunner := service.ProvideChannelMonitorRunner(channelMonitorSvc, settingSvc)
	channelMonitorTemplateSvc := service.NewChannelMonitorRequestTemplateService(channelMonitorTemplateRepo)

	backupSvc := service.ProvideBackupService(settingRepo, cfg, aesEncryptor, s3BackupStoreFactory, pgDumper)

	opsSystemLogSink := service.ProvideOpsSystemLogSink(opsRepo)
	opsSvc := service.NewOpsService(
		opsRepo, settingRepo, cfg, accountRepo, userRepo,
		concurrencySvc, gatewaySvc, openAIGatewaySvc,
		geminiCompatSvc, antigravityGatewaySvc, opsSystemLogSink,
	)
	opsMetricsCollector := service.ProvideOpsMetricsCollector(opsRepo, settingRepo, accountRepo, concurrencySvc, db, redisStub, cfg)
	opsAggregationSvc := service.ProvideOpsAggregationService(opsRepo, settingRepo, db, redisStub, cfg)
	opsAlertEvaluatorSvc := service.ProvideOpsAlertEvaluatorService(opsSvc, opsRepo, emailSvc, redisStub, cfg)
	opsCleanupSvc := service.ProvideOpsCleanupService(opsRepo, db, redisStub, cfg, channelMonitorSvc, settingRepo, opsSvc)
	opsScheduledReportSvc := service.ProvideOpsScheduledReportService(opsSvc, userSvc, emailSvc, redisStub, cfg)

	privacyClientFactory := service.PrivacyClientFactory(nil)

	adminSvc := service.NewAdminService(
		userRepo, groupRepo, accountRepo, proxyRepo,
		apiKeyRepo, redeemCodeRepo, userGroupRateRepo,
		userRPMCache, billingCacheSvc, proxyExitInfoProber,
		proxyLatencyCache, authCacheInvalidator, entClient,
		settingSvc, subscriptionSvc, userSubRepo,
		privacyClientFactory, openAIGatewaySvc,
	)

	authSvc := service.NewAuthService(
		entClient, userRepo, redeemCodeRepo, refreshTokenCache,
		cfg, settingSvc, emailSvc, turnstileSvc, emailQueueSvc,
		promoSvc, subscriptionSvc, affiliateSvc,
	)

	tokenRefreshSvc := service.ProvideTokenRefreshService(
		accountRepo, oauthSvc, openAIOAuthSvc, geminiOAuthSvc,
		antigravityOAuthSvc, tokenCacheInvalidator, schedulerCache,
		cfg, tempUnschedCache, privacyClientFactory, proxyRepo,
		oauthRefreshAPI, openAIGatewaySvc,
	)

	updateSvc := service.ProvideUpdateService(updateCache, githubReleaseClient, service.BuildInfo{Version: "dev", BuildType: "source"})

	_ = accountExpirySvc
	_ = subscriptionExpirySvc
	_ = deferredSvc
	_ = schedulerSnapshotSvc
	_ = usageCleanupSvc
	_ = dashboardAggSvc
	_ = idempotencyCoordinator
	_ = idempotencyCleanupSvc
	_ = scheduledTestRunnerSvc
	_ = paymentOrderExpirySvc
	_ = channelMonitorRunner
	_ = opsMetricsCollector
	_ = opsAggregationSvc
	_ = opsAlertEvaluatorSvc
	_ = opsCleanupSvc
	_ = opsScheduledReportSvc
	_ = tokenRefreshSvc
	_ = backupSvc
	_ = systemOpLockSvc

	jwtAuth := middleware.NewJWTAuthMiddleware(authSvc, userSvc)
	adminAuth := middleware.NewAdminAuthMiddleware(authSvc, userSvc, settingSvc)
	apiKeyAuth := middleware.NewAPIKeyAuthMiddleware(apiKeySvc, subscriptionSvc, cfg)

	adminHandlers := handler.ProvideAdminHandlers(
		admin.NewDashboardHandler(dashboardSvc, dashboardAggSvc),
		admin.NewUserHandler(adminSvc, concurrencySvc),
		admin.NewGroupHandler(adminSvc, dashboardSvc, groupCapacitySvc),
		admin.NewAccountHandler(
			adminSvc, oauthSvc, openAIOAuthSvc, geminiOAuthSvc,
			antigravityOAuthSvc, rateLimitSvc, accountUsageSvc,
			accountTestSvc, concurrencySvc, crsSyncSvc,
			sessionLimitCache, rpmCache, tokenCacheInvalidator,
		),
		admin.NewAnnouncementHandler(announcementSvc),
		admin.NewDataManagementHandler(dataManagementSvc),
		admin.NewBackupHandler(backupSvc, userSvc),
		admin.NewOAuthHandler(oauthSvc),
		admin.NewOpenAIOAuthHandler(openAIOAuthSvc, adminSvc),
		admin.NewGeminiOAuthHandler(geminiOAuthSvc),
		admin.NewAntigravityOAuthHandler(antigravityOAuthSvc),
		admin.NewProxyHandler(adminSvc),
		admin.NewRedeemHandler(adminSvc, redeemSvc),
		admin.NewPromoHandler(promoSvc),
		handler.ProvideAdminSettingHandler(settingSvc, emailSvc, turnstileSvc, opsSvc, paymentConfigSvc, paymentSvc, userAttrSvc, notificationEmailSvc),
		admin.NewOpsHandler(opsSvc),
		handler.ProvideSystemHandler(updateSvc, systemOpLockSvc),
		admin.NewSubscriptionHandler(subscriptionSvc),
		admin.NewUsageHandler(usageSvc, apiKeySvc, adminSvc, usageCleanupSvc),
		admin.NewUserAttributeHandler(userAttrSvc),
		admin.NewErrorPassthroughHandler(errorPassthroughSvc),
		admin.NewTLSFingerprintProfileHandler(tlsFPProfileSvc),
		admin.NewAdminAPIKeyHandler(adminSvc),
		admin.NewScheduledTestHandler(scheduledTestSvc),
		admin.NewChannelHandler(channelSvc, billingSvc, pricingSvc),
		admin.NewChannelMonitorHandler(channelMonitorSvc),
		admin.NewChannelMonitorRequestTemplateHandler(channelMonitorTemplateSvc),
		admin.NewContentModerationHandler(contentModerationSvc),
		admin.NewPaymentHandler(paymentSvc, paymentConfigSvc),
		admin.NewAffiliateHandler(affiliateSvc, adminSvc),
	)

	handlers := handler.ProvideHandlers(
		handler.NewAuthHandler(cfg, authSvc, userSvc, settingSvc, promoSvc, redeemSvc, totpSvc, userAttrSvc),
		handler.NewUserHandler(userSvc, authSvc, emailSvc, emailCache, affiliateSvc),
		handler.NewAPIKeyHandler(apiKeySvc),
		handler.NewUsageHandler(usageSvc, apiKeySvc),
		handler.NewRedeemHandler(redeemSvc),
		handler.NewSubscriptionHandler(subscriptionSvc),
		handler.NewAnnouncementHandler(announcementSvc),
		handler.NewChannelMonitorUserHandler(channelMonitorSvc, settingSvc),
		adminHandlers,
		handler.NewGatewayHandler(
			gatewaySvc, geminiCompatSvc, antigravityGatewaySvc,
			userSvc, concurrencySvc, billingCacheSvc, usageSvc,
			apiKeySvc, usageRecordWorkerPool, errorPassthroughSvc,
			contentModerationSvc, userMsgQueueSvc, cfg, settingSvc,
		),
		handler.NewOpenAIGatewayHandler(
			openAIGatewaySvc, concurrencySvc, billingCacheSvc,
			apiKeySvc, usageRecordWorkerPool, errorPassthroughSvc,
			contentModerationSvc, cfg,
		),
		handler.ProvideSettingHandler(settingSvc, handler.BuildInfo{Version: "dev", BuildType: "source"}, notificationEmailSvc),
		handler.NewTotpHandler(totpSvc),
		handler.NewPaymentHandler(paymentSvc, paymentConfigSvc, channelSvc),
		handler.NewPaymentWebhookHandler(paymentSvc, paymentRegistry),
		handler.NewAvailableChannelHandler(channelSvc, apiKeySvc, settingSvc),
		idempotencyCoordinator,
		idempotencyCleanupSvc,
	)

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	if len(cfg.Server.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
			log.Printf("Failed to set trusted proxies: %v", err)
		}
	} else {
		if err := r.SetTrustedProxies(nil); err != nil {
			log.Printf("Failed to disable trusted proxies: %v", err)
		}
	}

	settingSvc.SetWebSearchManagerBuilder(context.Background(), func(wscfg *service.WebSearchEmulationConfig, proxyURLs map[int64]string) {
		if wscfg == nil || !wscfg.Enabled || len(wscfg.Providers) == 0 {
			service.SetWebSearchManager(nil)
			return
		}
		configs := make([]websearch.ProviderConfig, 0, len(wscfg.Providers))
		for _, p := range wscfg.Providers {
			if p.APIKey == "" {
				continue
			}
			pc := websearch.ProviderConfig{
				Type:       p.Type,
				APIKey:     p.APIKey,
				QuotaLimit: derefInt64(p.QuotaLimit),
				ExpiresAt:  p.ExpiresAt,
			}
			if p.SubscribedAt != nil {
				pc.SubscribedAt = p.SubscribedAt
			}
			if p.ProxyID != nil {
				pc.ProxyID = *p.ProxyID
				if u, ok := proxyURLs[*p.ProxyID]; ok {
					pc.ProxyURL = u
				}
			}
			configs = append(configs, pc)
		}
		service.SetWebSearchManager(websearch.NewManager(configs, redisStub))
	})

	antigravity.SetUserAgentVersionResolver(settingSvc.GetAntigravityUserAgentVersion)

	SetupRouter(r, handlers, jwtAuth, adminAuth, apiKeyAuth, apiKeySvc, subscriptionSvc, opsSvc, settingSvc, cfg, redisStub)

	httpHandler := http.Handler(r)
	srv := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           httpHandler,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	globalMaxSize := cfg.Server.MaxRequestBodySize
	if globalMaxSize <= 0 {
		globalMaxSize = cfg.Gateway.MaxBodySize
	}
	if globalMaxSize > 0 {
		httpHandler = http.MaxBytesHandler(httpHandler, globalMaxSize)
		log.Printf("Global max request body size: %d bytes (%.2f MB)", globalMaxSize, float64(globalMaxSize)/(1<<20))
	}
	srv.Handler = httpHandler

	return srv, nil
}
