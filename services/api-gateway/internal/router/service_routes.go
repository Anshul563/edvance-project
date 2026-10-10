package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/api-gateway/internal/config"
	"github.com/Anshul563/edvance-project/services/api-gateway/internal/proxy"
)

func registerServiceRoutes(
	r chi.Router,
	cfg config.Config,
) error {

	authProxy, err := proxy.New(
		cfg.Services.AuthURL,
		"/api/v1/auth",
	)
	if err != nil {
		return err
	}

	userProxy, err := proxy.New(
		cfg.Services.UserURL,
		"/api/v1/users",
	)
	if err != nil {
		return err
	}

	creatorProxy, err := proxy.New(
		cfg.Services.CreatorURL,
		"/api/v1/creators",
	)
	if err != nil {
		return err
	}

	contentProxy, err := proxy.New(
		cfg.Services.ContentURL,
		"/api/v1/content",
	)
	if err != nil {
		return err
	}

	videoProxy, err := proxy.New(
		cfg.Services.VideoURL,
		"/api/v1/videos",
	)
	if err != nil {
		return err
	}

	mediaProxy, err := proxy.New(
		cfg.Services.MediaURL,
		"/api/v1/media",
	)
	if err != nil {
		return err
	}

	// Course-service owns three mounts, so all three strip only /api/v1:
	// the service sees distinguishing /courses/*, /sections/*, and
	// /lessons/* paths. Stripping each mount fully would collapse
	// /api/v1/sections/:id and /api/v1/courses/:id into the same /:id.
	courseProxy, err := proxy.New(
		cfg.Services.CourseURL,
		"/api/v1",
	)
	if err != nil {
		return err
	}

	sectionProxy, err := proxy.New(
		cfg.Services.CourseURL,
		"/api/v1",
	)
	if err != nil {
		return err
	}

	lessonProxy, err := proxy.New(
		cfg.Services.CourseURL,
		"/api/v1",
	)
	if err != nil {
		return err
	}

	learningProxy, err := proxy.New(
		cfg.Services.LearningURL,
		"/api/v1/learning",
	)
	if err != nil {
		return err
	}

	socialProxy, err := proxy.New(
		cfg.Services.SocialURL,
		"/api/v1/social",
	)
	if err != nil {
		return err
	}

	commerceProxy, err := proxy.New(
		cfg.Services.CommerceURL,
		"/api/v1/commerce",
	)
	if err != nil {
		return err
	}

	paymentProxy, err := proxy.New(
		cfg.Services.PaymentURL,
		"/api/v1/payments",
	)
	if err != nil {
		return err
	}

	notificationProxy, err := proxy.New(
		cfg.Services.NotificationURL,
		"/api/v1/notifications",
	)
	if err != nil {
		return err
	}

	// Search-service owns discovery. The gateway forwards
	// /api/v1/search/* with the prefix stripped, so the
	// service sees /?q=... — it also serves the full path
	// for direct callers.
	searchProxy, err := proxy.New(
		cfg.Services.SearchURL,
		"/api/v1/search",
	)
	if err != nil {
		return err
	}

	recommendationProxy, err := proxy.New(
		cfg.Services.RecommendationURL,
		"/api/v1/recommendations",
	)
	if err != nil {
		return err
	}

	moderationProxy, err := proxy.New(
		cfg.Services.ModerationURL,
		"/api/v1/moderation",
	)
	if err != nil {
		return err
	}

	aiProxy, err := proxy.New(
		cfg.Services.AIURL,
		"/api/v1/ai",
	)
	if err != nil {
		return err
	}

	liveProxy, err := proxy.New(
		cfg.Services.LiveStreamingURL,
		"/api/v1/live",
	)
	if err != nil {
		return err
	}

	streamProxy, err := proxy.New(
		cfg.Services.LiveStreamingURL,
		"/api/v1/streams",
	)
	if err != nil {
		return err
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", authProxy)
		r.Mount("/users", userProxy)
		r.Mount("/creators", creatorProxy)
		r.Mount("/content", contentProxy)
		r.Mount("/videos", videoProxy)
		r.Mount("/media", mediaProxy)
		r.Mount("/courses", courseProxy)
		r.Mount("/sections", sectionProxy)
		r.Mount("/lessons", lessonProxy)
		r.Mount("/learning", learningProxy)
		r.Mount("/social", socialProxy)
		r.Mount("/commerce", commerceProxy)
		r.Mount("/payments", paymentProxy)
		// NOTE: /internal/* is deliberately never mounted. Internal
		// service routes stay on private networking only.
		r.Mount("/notifications", notificationProxy)
		r.Mount("/search", searchProxy)
		r.Mount("/recommendations", recommendationProxy)
		r.Mount("/moderation", moderationProxy)
		r.Mount("/ai", aiProxy)
		r.Mount("/live", liveProxy)
		r.Mount("/streams", streamProxy)
	})

	return nil
}

func serviceNotFound(w http.ResponseWriter, r *http.Request) {
	http.Error(
		w,
		"service route not found",
		http.StatusNotFound,
	)
}
