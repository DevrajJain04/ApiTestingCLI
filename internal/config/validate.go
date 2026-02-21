package config

import (
	"fmt"
	"strings"

	"github.com/DevrajJain04/reqres/internal/model"
)

func Validate(cfg model.Config) []error {
	var errs []error
	if strings.TrimSpace(cfg.Base) == "" {
		errs = append(errs, fmt.Errorf("base is required"))
	}
	if cfg.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("timeout must be > 0"))
	}
	if cfg.Retries < 0 {
		errs = append(errs, fmt.Errorf("retries must be >= 0"))
	}

	nameSeen := map[string]struct{}{}
	for i, test := range cfg.Tests {
		location := fmt.Sprintf("tests[%d]", i)
		if strings.TrimSpace(test.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", location))
		} else {
			if _, ok := nameSeen[test.Name]; ok {
				errs = append(errs, fmt.Errorf("duplicate test name %q", test.Name))
			}
			nameSeen[test.Name] = struct{}{}
		}
		if strings.TrimSpace(test.Path) == "" {
			errs = append(errs, fmt.Errorf("%s.path is required", location))
		}
		if test.Retries != nil && *test.Retries < 0 {
			errs = append(errs, fmt.Errorf("%s.retries must be >= 0", location))
		}
		if test.TimeoutMS != nil && *test.TimeoutMS <= 0 {
			errs = append(errs, fmt.Errorf("%s.timeout must be > 0", location))
		}
	}

	for _, test := range cfg.Tests {
		if test.After == "" {
			continue
		}
		if _, ok := nameSeen[test.After]; !ok {
			errs = append(errs, fmt.Errorf("test %q depends on unknown test %q", test.Name, test.After))
		}
	}

	if cfg.Load != nil {
		if cfg.Load.Users <= 0 {
			errs = append(errs, fmt.Errorf("load.users must be > 0"))
		}
		if strings.TrimSpace(cfg.Load.Duration) == "" {
			errs = append(errs, fmt.Errorf("load.duration is required"))
		}
	}

	if cfg.Mock != nil {
		for i, route := range cfg.Mock.Routes {
			if strings.TrimSpace(route.Path) == "" {
				errs = append(errs, fmt.Errorf("mock.routes[%d].path is required", i))
			}
			if route.Status <= 0 {
				errs = append(errs, fmt.Errorf("mock.routes[%d].status must be > 0", i))
			}
		}
	}
	return errs
}
