package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// PlaywrightAdapter implements browser automation via playwright-go.
type PlaywrightAdapter struct {
	browser playwright.Browser
	page    playwright.Page
	pw      *playwright.Playwright
	baseURL string
	pages   []playwright.Page
	timeout float64
}

func NewPlaywrightAdapter() *PlaywrightAdapter {
	return &PlaywrightAdapter{
		timeout: 5000,
	}
}

func (a *PlaywrightAdapter) Init(config map[string]string) error {
	url, ok := config["base_url"]
	if !ok {
		return errors.New("playwright adapter requires base_url in target config")
	}
	a.baseURL = url

	headless := true
	if v, ok := config["headless"]; ok && v == "false" {
		headless = false
	}

	if v, ok := config["timeout"]; ok {
		t, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", v, err)
		}
		a.timeout = t
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf(
			"starting playwright: %w\n\nHint: run 'specrun install playwright' to install browsers",
			err,
		)
	}
	a.pw = pw

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		a.pw.Stop() //nolint:errcheck
		return fmt.Errorf(
			"launching browser: %w\n\nHint: run 'specrun install playwright' to install browsers",
			err,
		)
	}
	a.browser = browser

	page, err := browser.NewPage()
	if err != nil {
		browser.Close() //nolint:errcheck
		a.pw.Stop()     //nolint:errcheck
		return fmt.Errorf("creating page: %w", err)
	}
	a.page = page
	a.pages = append(a.pages, page)

	return nil
}

func (a *PlaywrightAdapter) Action(name string, args json.RawMessage) (*Response, error) {
	var rawArgs []json.RawMessage
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawArgs); err != nil {
			return nil, fmt.Errorf("parsing action args: %w", err)
		}
	}

	switch name {
	case "goto":
		return a.doGoto(rawArgs)
	case "click":
		return a.doClick(rawArgs)
	case "fill":
		return a.doFill(rawArgs)
	case "type":
		return a.doType(rawArgs)
	case "select":
		return a.doSelect(rawArgs)
	case "check":
		return a.doCheck(rawArgs)
	case "uncheck":
		return a.doUncheck(rawArgs)
	case "wait":
		return a.doWait(rawArgs)
	case "resize":
		return a.doResize(rawArgs)
	case "new_page":
		return a.doNewPage()
	case "close_page":
		return a.doClosePage()
	case "clear_state":
		return a.doClearState()
	default:
		return nil, fmt.Errorf("unknown playwright action %q", name)
	}
}

func (a *PlaywrightAdapter) Assert(
	property string,
	locator string,
	expected json.RawMessage,
) (*Response, error) {
	if a.page == nil {
		return nil, errors.New("no page available for assertion")
	}

	loc := a.page.Locator(locator)
	timeout := a.timeout

	var actual any
	var err error

	switch {
	case property == "visible":
		actual, err = loc.IsVisible(playwright.LocatorIsVisibleOptions{Timeout: &timeout})
	case property == "text":
		actual, err = loc.TextContent(playwright.LocatorTextContentOptions{Timeout: &timeout})
	case property == "value":
		actual, err = loc.InputValue(playwright.LocatorInputValueOptions{Timeout: &timeout})
	case property == "checked":
		actual, err = loc.IsChecked(playwright.LocatorIsCheckedOptions{Timeout: &timeout})
	case property == "disabled":
		actual, err = loc.IsDisabled(playwright.LocatorIsDisabledOptions{Timeout: &timeout})
	case property == "count":
		actual, err = loc.Count()
	case strings.HasPrefix(property, "attribute."):
		attrName := strings.TrimPrefix(property, "attribute.")
		actual, err = loc.GetAttribute(
			attrName,
			playwright.LocatorGetAttributeOptions{Timeout: &timeout},
		)
	default:
		return nil, fmt.Errorf("unknown playwright assertion property %q", property)
	}

	if err != nil {
		return &Response{
			OK:    false,
			Error: fmt.Sprintf("assertion %q on %q: %v", property, locator, err),
		}, nil
	}

	// Normalize both sides through JSON for consistent comparison.
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return nil, fmt.Errorf("marshaling actual value: %w", err)
	}

	var actualNorm, expectedNorm any
	if err := json.Unmarshal(actualJSON, &actualNorm); err != nil {
		return nil, fmt.Errorf("normalizing actual: %w", err)
	}
	if err := json.Unmarshal(expected, &expectedNorm); err != nil {
		return nil, fmt.Errorf("normalizing expected: %w", err)
	}

	if reflect.DeepEqual(actualNorm, expectedNorm) {
		return &Response{OK: true, Actual: actualJSON}, nil
	}

	return &Response{
		OK:     false,
		Actual: actualJSON,
		Error:  fmt.Sprintf("expected %s, got %s", string(expected), string(actualJSON)),
	}, nil
}

func (a *PlaywrightAdapter) Close() error {
	var errs []error
	for _, p := range a.pages {
		if err := p.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	a.pages = nil
	a.page = nil

	if a.browser != nil {
		if err := a.browser.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.pw != nil {
		if err := a.pw.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// --- Action implementations ---

func (a *PlaywrightAdapter) doGoto(args []json.RawMessage) (*Response, error) {
	if len(args) < 1 {
		return nil, errors.New("goto requires a URL argument")
	}
	var url string
	if err := json.Unmarshal(args[0], &url); err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	// Prepend base URL for relative paths.
	if strings.HasPrefix(url, "/") {
		url = a.baseURL + url
	}

	if _, err := a.page.Goto(url); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doClick(args []json.RawMessage) (*Response, error) {
	selector, err := a.parseSelector(args)
	if err != nil {
		return nil, err
	}
	if err := a.page.Locator(selector).Click(playwright.LocatorClickOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doFill(args []json.RawMessage) (*Response, error) {
	if len(args) < 2 {
		return nil, errors.New("fill requires selector and value arguments")
	}
	var selector, value string
	if err := json.Unmarshal(args[0], &selector); err != nil {
		return nil, fmt.Errorf("parsing selector: %w", err)
	}
	if err := json.Unmarshal(args[1], &value); err != nil {
		return nil, fmt.Errorf("parsing value: %w", err)
	}
	if err := a.page.Locator(selector).Fill(value, playwright.LocatorFillOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doType(args []json.RawMessage) (*Response, error) {
	if len(args) < 2 {
		return nil, errors.New("type requires selector and value arguments")
	}
	var selector, value string
	if err := json.Unmarshal(args[0], &selector); err != nil {
		return nil, fmt.Errorf("parsing selector: %w", err)
	}
	if err := json.Unmarshal(args[1], &value); err != nil {
		return nil, fmt.Errorf("parsing value: %w", err)
	}
	if err := a.page.Locator(selector).
		PressSequentially(value, playwright.LocatorPressSequentiallyOptions{
			Timeout: &a.timeout,
		}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doSelect(args []json.RawMessage) (*Response, error) {
	if len(args) < 2 {
		return nil, errors.New("select requires selector and value arguments")
	}
	var selector, value string
	if err := json.Unmarshal(args[0], &selector); err != nil {
		return nil, fmt.Errorf("parsing selector: %w", err)
	}
	if err := json.Unmarshal(args[1], &value); err != nil {
		return nil, fmt.Errorf("parsing value: %w", err)
	}
	if _, err := a.page.Locator(selector).SelectOption(playwright.SelectOptionValues{
		Values: &[]string{value},
	}, playwright.LocatorSelectOptionOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doCheck(args []json.RawMessage) (*Response, error) {
	selector, err := a.parseSelector(args)
	if err != nil {
		return nil, err
	}
	if err := a.page.Locator(selector).Check(playwright.LocatorCheckOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doUncheck(args []json.RawMessage) (*Response, error) {
	selector, err := a.parseSelector(args)
	if err != nil {
		return nil, err
	}
	if err := a.page.Locator(selector).Uncheck(playwright.LocatorUncheckOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doWait(args []json.RawMessage) (*Response, error) {
	selector, err := a.parseSelector(args)
	if err != nil {
		return nil, err
	}
	if err := a.page.Locator(selector).WaitFor(playwright.LocatorWaitForOptions{
		Timeout: &a.timeout,
	}); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doResize(args []json.RawMessage) (*Response, error) {
	if len(args) < 2 {
		return nil, errors.New("resize requires width and height arguments")
	}
	var width, height int
	if err := json.Unmarshal(args[0], &width); err != nil {
		return nil, fmt.Errorf("parsing width: %w", err)
	}
	if err := json.Unmarshal(args[1], &height); err != nil {
		return nil, fmt.Errorf("parsing height: %w", err)
	}
	if err := a.page.SetViewportSize(width, height); err != nil {
		return &Response{OK: false, Error: err.Error()}, nil
	}
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doNewPage() (*Response, error) {
	page, err := a.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("creating new page: %w", err)
	}
	a.page = page
	a.pages = append(a.pages, page)
	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doClosePage() (*Response, error) {
	if a.page == nil {
		return &Response{OK: true}, nil
	}
	if err := a.page.Close(); err != nil {
		return nil, fmt.Errorf("closing page: %w", err)
	}

	// Remove closed page from tracking and switch to the last remaining page.
	for i, p := range a.pages {
		if p == a.page {
			a.pages = append(a.pages[:i], a.pages[i+1:]...)
			break
		}
	}

	if len(a.pages) > 0 {
		a.page = a.pages[len(a.pages)-1]
	} else {
		a.page = nil
	}

	return &Response{OK: true}, nil
}

func (a *PlaywrightAdapter) doClearState() (*Response, error) {
	ctx := a.browser.Contexts()
	if len(ctx) > 0 {
		if err := ctx[0].ClearCookies(); err != nil {
			return &Response{OK: false, Error: err.Error()}, nil
		}
		// Clear localStorage on all pages.
		for _, p := range a.pages {
			if _, err := p.Evaluate("() => localStorage.clear()"); err != nil {
				// Best-effort — page might not have navigated yet.
				continue
			}
		}
	}
	return &Response{OK: true}, nil
}

// parseSelector extracts a single selector string from the first argument.
func (*PlaywrightAdapter) parseSelector(args []json.RawMessage) (string, error) {
	if len(args) < 1 {
		return "", errors.New("action requires a selector argument")
	}
	var selector string
	if err := json.Unmarshal(args[0], &selector); err != nil {
		return "", fmt.Errorf("parsing selector: %w", err)
	}
	return selector, nil
}
