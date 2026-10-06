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

	courseProxy, err := proxy.New(
		cfg.Services.CourseURL,
		"/api/v1/courses",
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

	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", authProxy)
		r.Mount("/users", userProxy)
		r.Mount("/creators", creatorProxy)
		r.Mount("/content", contentProxy)
		r.Mount("/videos", videoProxy)
		r.Mount("/courses", courseProxy)
		r.Mount("/learning", learningProxy)
		r.Mount("/social", socialProxy)
		r.Mount("/commerce", commerceProxy)
		r.Mount("/payments", paymentProxy)
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