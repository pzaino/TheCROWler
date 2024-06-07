// Copyright 2023 Paolo Fabio Zaino
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package crawler implements the crawling logic of the application.
// It's responsible for crawling a website and extracting information from it.
package crawler

import (
	"fmt"
	"strings"
	"time"

	cmn "github.com/pzaino/thecrowler/pkg/common"
	cfg "github.com/pzaino/thecrowler/pkg/config"
	rules "github.com/pzaino/thecrowler/pkg/ruleset"
	"github.com/tebeka/selenium"
)

func processActionRules(wd *selenium.WebDriver, ctx *processContext, url string) {
	cmn.DebugMsg(cmn.DbgLvlDebug2, "Starting to search and process CROWler Action rules...")
	// Run Action Rules if any
	if ctx.source.Config != nil {
		// Execute the CROWler rules
		cmn.DebugMsg(cmn.DbgLvlDebug, "Executing CROWler configured Action rules...")
		// Execute the rules
		if strings.TrimSpace(string((*ctx.source.Config))) == "{\"config\":\"default\"}" {
			runDefaultActionRules(wd, ctx)
		} else {
			configStr := string((*ctx.source.Config))
			cmn.DebugMsg(cmn.DbgLvlDebug, "Configuration: %v", configStr)
		}
	} else {
		// Check for rules based on the URL
		cmn.DebugMsg(cmn.DbgLvlDebug, "Executing CROWler URL based Action rules...")
		// If the URL matches a rule, execute it
		processURLRules(wd, ctx, url)
	}
}

func processURLRules(wd *selenium.WebDriver, ctx *processContext, url string) {
	rs, err := ctx.re.GetRulesetByURL(url)
	if err == nil {
		if rs != nil {
			cmn.DebugMsg(cmn.DbgLvlDebug, "Executing ruleset: %s", rs.Name)
			// Execute all the rules in the ruleset
			executeActionRules(ctx, rs.GetAllEnabledActionRules(), wd)
		}
	} else {
		rg, err := ctx.re.GetRuleGroupByURL(url)
		if err == nil {
			if rg != nil {
				cmn.DebugMsg(cmn.DbgLvlDebug, "Executing rule group: %s", rg.GroupName)
				// Execute all the rules in the rule group
				executeActionRules(ctx, rg.GetActionRules(), wd)
			}
		}
	}
}

func executeActionRules(ctx *processContext,
	rules []rules.ActionRule, wd *selenium.WebDriver) {
	// Extract each rule and execute it
	for _, r := range rules {
		executeRule(ctx, &r, wd)
		ctx.Status.TotalActions++
	}
}

func executeRule(ctx *processContext, r *rules.ActionRule, wd *selenium.WebDriver) {
	// Execute the rule
	err := executeActionRule(ctx, r, wd)
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlError, "executing action rule: %v", err)
		if !r.ErrorHandling.Ignore {
			if r.ErrorHandling.RetryCount > 0 {
				for i := 0; i < r.ErrorHandling.RetryCount; i++ {
					if r.ErrorHandling.RetryDelay > 0 {
						time.Sleep(time.Duration(r.ErrorHandling.RetryDelay) * time.Second)
					}
					err = executeActionRule(ctx, r, wd)
					if err == nil {
						break
					}
				}
			}
		}
	}
}

// executeActionRule executes a single ActionRule
func executeActionRule(ctx *processContext, r *rules.ActionRule, wd *selenium.WebDriver) error {
	// Execute Wait condition first
	if len(r.WaitConditions) != 0 {
		for _, wc := range r.WaitConditions {
			// Execute the wait condition
			err := executeWaitCondition(ctx, &wc, wd)
			if err != nil {
				return err
			}
		}
	}
	// Execute the action based on the ActionType
	if (len(r.Conditions) == 0) || checkActionConditions(ctx, r.Conditions, wd) {
		switch strings.ToLower(strings.TrimSpace(r.ActionType)) {
		case "click":
			return executeActionClick(r, wd)
		case "scroll":
			return executeActionScroll(r, wd)
		case "input_text":
			return executeActionInput(r, wd)
		case "clear":
			return executeActionClear(r, wd)
		case "custom":
			return executeActionJS(ctx, r, wd)
		case "take_screenshot":
			return executeActionScreenshot(r, wd)
		case "key_down":
			return executeActionKeyDown(r, wd)
		case "key_up":
			return executeActionKeyUp(r, wd)
		case "mouse_hover":
			return executeActionMouseHover(r, wd)
		case "forward":
			return executeActionForward(wd)
		case "back":
			return executeActionBack(wd)
		case "refresh":
			return executeActionRefresh(wd)
		case "switch_to_frame":
			return executeActionSwitchFrame(r, wd)
		case "switch_to_window":
			return executeActionSwitchWindow(r, wd)
		case "scroll_to_element":
			return executeActionScrollToElement(r, wd)
		case "scroll_by_amount":
			return executeActionScrollByAmount(r, wd)
		case "click_and_hold":
			return executeActionClickAndHold(r, wd)
		case "release":
			return executeActionRelease(r, wd)
		case "navigate_to_url":
			return executeActionNavigateToURL(r, wd)
		}
		return fmt.Errorf("action type not supported: %s", r.ActionType)
	}
	return nil
}

func executeActionNavigateToURL(r *rules.ActionRule, wd *selenium.WebDriver) error {
	return (*wd).Get(r.GetValue())
}

func executeActionClickAndHold(r *rules.ActionRule, wd *selenium.WebDriver) error {
	wdf, _, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		return err
	}
	// JavaScript to simulate a click and hold
	id, err := wdf.GetAttribute("id")
	if err != nil {
		id, err = wdf.GetAttribute("name")
		if err != nil {
			return err
		}
	}
	script := `
		var elem = document.getElementById('` + id + `');
		var evt1 = new MouseEvent('mousemove', {
			bubbles: true,
			cancelable: true,
			clientX: elem.getBoundingClientRect().left,
			clientY: elem.getBoundingClientRect().top,
			view: window
		});
		var evt2 = new MouseEvent('mousedown', {
			bubbles: true,
			cancelable: true,
			clientX: elem.getBoundingClientRect().left,
			clientY: elem.getBoundingClientRect().top,
			view: window
		});
		elem.dispatchEvent(evt1);
		elem.dispatchEvent(evt2);
	`
	_, err = (*wd).ExecuteScript(script, nil)
	return err
}

func executeActionRelease(r *rules.ActionRule, wd *selenium.WebDriver) error {
	var element selenium.WebElement
	if r.Selectors != nil {
		element, _, _ = findElementBySelectorType(wd, r.Selectors)
	}
	var script string
	if element != nil {
		id, err := element.GetAttribute("id")
		if err != nil {
			id, err = element.GetAttribute("name")
			if err != nil {
				return err
			}
		}
		script = `
			var elem = document.getElementById('` + id + `');
			var evt3 = new MouseEvent('mouseup', {
				bubbles: true,
				cancelable: true,
				clientX: elem.getBoundingClientRect().left,
				clientY: elem.getBoundingClientRect().top,
				view: window
			});
			elem.dispatchEvent(evt3);
		`
	} else {
		// Get the element at the current mouse coordinates:
		script = `
			const x = event.clientX;
			const y = event.clientY;
			elem = document.elementFromPoint(x, y);
			var evt3 = new MouseEvent('mouseup', {
				bubbles: true,
				cancelable: true,
				clientX: elem.getBoundingClientRect().left,
				clientY: elem.getBoundingClientRect().top,
				view: window
			});
			elem.dispatchEvent(evt3);
		`
	}
	_, err := (*wd).ExecuteScript(script, nil)
	return err
}

func executeActionClear(r *rules.ActionRule, wd *selenium.WebDriver) error {
	wdf, _, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		return err
	}
	return wdf.Clear()
}

// executeActionScreenshot is responsible for executing a "take_screenshot" action
// It takes a screenshot of the current page and saves it to the configured location
// r.Value contains the filename of the screenshot and the max height of the screenshot
// (optional, if not provided the screenshot will be taken of the entire page)
// rValue syntax is: "maxHeight,fileName"
func executeActionScreenshot(r *rules.ActionRule, wd *selenium.WebDriver) error {
	// Check if the rule contains also a max height
	val := r.GetValue()
	hVal := ""
	fVal := ""
	if strings.Contains(val, ",") {
		hVal = strings.Split(val, ",")[0]
		fVal = strings.Split(val, ",")[1]
	} else {
		hVal = "0"
		fVal = val
	}
	hInt := cmn.StringToInt(hVal)

	_, err := TakeScreenshot(wd, fVal, hInt)
	return err
}

func executeActionKeyDown(r *rules.ActionRule, wd *selenium.WebDriver) error {
	return (*wd).KeyDown(r.Value)
}

func executeActionKeyUp(r *rules.ActionRule, wd *selenium.WebDriver) error {
	return (*wd).KeyUp(r.Value)
}

func executeActionMouseHover(r *rules.ActionRule, wd *selenium.WebDriver) error {
	return executeMoveToElement(r, wd)
}

func executeActionForward(wd *selenium.WebDriver) error {
	return (*wd).Forward()
}

func executeActionBack(wd *selenium.WebDriver) error {
	return (*wd).Back()
}

func executeActionRefresh(wd *selenium.WebDriver) error {
	return (*wd).Refresh()
}

func executeActionSwitchFrame(r *rules.ActionRule, wd *selenium.WebDriver) error {
	wdf, _, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		return err
	}
	return (*wd).SwitchFrame(wdf)
}

func executeActionSwitchWindow(r *rules.ActionRule, wd *selenium.WebDriver) error {
	return (*wd).SwitchWindow(r.Value)
}

// TODO: Implement this function (this requires RBee service running on the VDI)
//
//	Scroll to an element using Rbee
func executeActionScrollToElement(_ *rules.ActionRule, _ *selenium.WebDriver) error {
	return nil
}

func executeActionScrollByAmount(r *rules.ActionRule, wd *selenium.WebDriver) error {
	y := cmn.StringToInt(r.Value)
	scrollScript := fmt.Sprintf("window.scrollTo(0, %d);", y)
	_, err := (*wd).ExecuteScript(scrollScript, nil)
	return err
}

// executeWaitCondition is responsible for executing a "wait" condition
func executeWaitCondition(ctx *processContext, r *rules.WaitCondition, wd *selenium.WebDriver) error {
	// Execute the wait condition
	switch strings.ToLower(strings.TrimSpace(r.ConditionType)) {
	case "element":
		return nil
	case "delay":
		return nil
	case "plugin_call":
		plugin, exists := ctx.re.JSPlugins.GetPlugin(r.Value)
		if !exists {
			return fmt.Errorf("plugin not found: %s", r.Value)
		}
		pluginCode := plugin.String()
		_, err := (*wd).ExecuteScript(pluginCode, nil)
		return err
	default:
		return fmt.Errorf("wait condition not supported: %s", r.ConditionType)
	}
}

// executeActionClick is responsible for executing a "click" action
func executeActionClick(r *rules.ActionRule, wd *selenium.WebDriver) error {
	// Find the element
	wdf, _, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlDebug3, "No element '%v' found.", err)
		err = nil
	}

	// If the element is found, click it
	if wdf != nil {
		err := wdf.Click()
		return err
	}
	return err
}

func executeMoveToElement(r *rules.ActionRule, wd *selenium.WebDriver) error {
	wdf, _, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		return err
	}
	id, err := wdf.GetAttribute("id")
	if err != nil {
		id, err = wdf.GetAttribute("name")
		if err != nil {
			return err
		}
	}
	script := `
		var elem = document.getElementById('` + id + `');
		var evt = new MouseEvent('mousemove', {
			bubbles: true,
			cancelable: true,
			clientX: elem.getBoundingClientRect().left,
			clientY: elem.getBoundingClientRect().top,
			view: window
		});
		elem.dispatchEvent(evt);
	`
	_, err = (*wd).ExecuteScript(script, nil)
	return err
}

// executeActionScroll is responsible for executing a "scroll" action
func executeActionScroll(r *rules.ActionRule, wd *selenium.WebDriver) error {
	// Get Selectors list
	value := r.Value

	// Get the attribute to scroll to
	var attribute string
	if value == "" {
		attribute = "document.body.scrollHeight"
	} else {
		attribute = value
	}

	// Use Sprintf to dynamically create the script string with the attribute value
	script := fmt.Sprintf("window.scrollTo(0, %s)", attribute)

	// Scroll the page
	_, err := (*wd).ExecuteScript(script, nil)
	return err
}

// executeActionJS is responsible for executing a "execute_javascript" action
func executeActionJS(ctx *processContext, r *rules.ActionRule, wd *selenium.WebDriver) error {
	for _, selector := range r.Selectors {
		if selector.SelectorType == "plugin_call" {
			// retrieve the JavaScript from the plugins registry using the value as the key
			plugin, exists := ctx.re.JSPlugins.GetPlugin(selector.Selector)
			if !exists {
				return fmt.Errorf("plugin not found: %s", selector.Selector)
			}

			// collect value as an argument to the plugin
			args := []interface{}{}
			args = append(args, r.Value)

			// Execute the JavaScript
			_, err := (*wd).ExecuteScript(plugin.String(), args)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// executeActionInput is responsible for executing an "input" action
func executeActionInput(r *rules.ActionRule, wd *selenium.WebDriver) error {
	// Find the element
	wdf, selector, err := findElementBySelectorType(wd, r.Selectors)
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlDebug3, "No element '%v' found.", err)
		err = nil
	}

	// If the element is found, input the text
	if wdf != nil {
		err = wdf.SendKeys(selector.Value)
	}
	return err
}

// findElementBySelectorType is responsible for finding an element in the WebDriver
// using the appropriate selector type. It returns the first element found and an error.
func findElementBySelectorType(wd *selenium.WebDriver, selectors []rules.Selector) (selenium.WebElement, rules.Selector, error) {
	var wdf selenium.WebElement = nil
	var err error
	var selector rules.Selector
	for _, selector = range selectors {
		wdf, err = findElementByType(wd, selector.SelectorType, selector.Selector)
		if err == nil && wdf != nil {
			matchL2 := false
			if strings.TrimSpace(selector.Attribute.Name) != "" {
				attrValue, _ := wdf.GetAttribute(strings.TrimSpace(selector.Attribute.Name))
				if strings.EqualFold(strings.TrimSpace(attrValue), strings.TrimSpace(selector.Attribute.Value)) {
					matchL2 = true
				}
			} else {
				matchL2 = true
			}
			matchL3 := false
			if matchL3 && strings.TrimSpace(selector.Value) != "" {
				if matchValue(wdf, selector) {
					matchL3 = true
				}
			} else {
				if matchL2 {
					matchL3 = true
				}
			}
			if matchL3 {
				break
			}
		}
	}

	return wdf, selector, err
}

func findElementByType(wd *selenium.WebDriver, selectorType string, selector string) (selenium.WebElement, error) {
	switch strings.ToLower(strings.TrimSpace(selectorType)) {
	case "css":
		return (*wd).FindElement(selenium.ByCSSSelector, selector)
	case "xpath":
		return (*wd).FindElement(selenium.ByXPATH, selector)
	case "id":
		return (*wd).FindElement(selenium.ByID, selector)
	case "name":
		return (*wd).FindElement(selenium.ByName, selector)
	case "linktext", "link_text":
		return (*wd).FindElement(selenium.ByLinkText, selector)
	case "partiallinktext", "partial_link_text":
		return (*wd).FindElement(selenium.ByPartialLinkText, selector)
	case "tagname", "tag_name", "tag", "element":
		return (*wd).FindElement(selenium.ByTagName, selector)
	case "class", "classname", "class_name":
		return (*wd).FindElement(selenium.ByClassName, selector)
	default:
		return nil, fmt.Errorf("unsupported selector type: %s", selectorType)
	}
}

func matchValue(wdf selenium.WebElement, selector rules.Selector) bool {
	// Precompute the common value for comparison
	wdfText, err := wdf.Text()
	if err != nil {
		return false
	}
	wdfText = strings.ToLower(strings.TrimSpace(wdfText))

	// Check if the selector value is one of the special cases
	selVal := strings.ToLower(strings.TrimSpace(selector.Value))
	if texts, ok := textMap[selVal]; ok {
		for _, val := range texts {
			if strings.Contains(wdfText, strings.ToLower(strings.TrimSpace(val))) {
				return true
			}
		}
		return false
	}

	// Generic comparison case
	if selVal != "" {
		if wdfText == selVal {
			return true
		}
	}
	return false
}

func DefaultActionConfig(url string) cfg.SourceConfig {
	return cfg.SourceConfig{
		FormatVersion: "1.0",
		Author:        "The CROWler team",
		CreatedAt:     time.Now(),
		Description:   "Default configuration",
		SourceName:    "Example Source",
		CrawlingConfig: cfg.CrawlingConfig{
			Site: url,
		},
		ExecutionPlan: []cfg.ExecutionPlanItem{
			{
				Label: "Default Execution Plan",
				Conditions: cfg.Condition{
					UrlPatterns: []string{url},
				},
				RuleGroups: []string{"CookieAcceptanceRulesExtended"},
			},
		},
	}
}

func runDefaultActionRules(wd *selenium.WebDriver, ctx *processContext) {
	// Execute the default scraping rules
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing default action rules...")

	// Get the default scraping rules
	url, err := (*wd).CurrentURL()
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlError, "getting the current URL: %v", err)
		url = ""
	}
	rs := DefaultActionConfig(url)
	// Check if the conditions are met
	if len(rs.ExecutionPlan) == 0 {
		cmn.DebugMsg(cmn.DbgLvlDebug, "No execution plan found for the current URL")
		return
	}
	// Execute all the rules in the ruleset
	for _, r := range rs.ExecutionPlan {
		// Check the conditions
		if !checkActionPreConditions(r.Conditions, url) {
			continue
		}
		if !checkActionConditions(ctx, r.AdditionalConditions, wd) {
			continue
		}
		if len(r.Rulesets) > 0 {
			executePlannedRulesets(wd, ctx, r)
		}
		if len(r.RuleGroups) > 0 {
			executePlannedRuleGroups(wd, ctx, r)
		}
		if len(r.Rules) > 0 {
			executePlannedRules(wd, ctx, r)
		}
	}
}

// checkActionPreConditions checks if the pre conditions are met
// for example if the page URL is listed in the list of URLs
// for which this rule is valid.
func checkActionPreConditions(conditions cfg.Condition, url string) bool {
	canProceed := true
	// Check the URL patterns
	if len(conditions.UrlPatterns) > 0 {
		for _, pattern := range conditions.UrlPatterns {
			if strings.Contains(url, pattern) {
				canProceed = true
			} else {
				canProceed = false
			}
		}
	}
	return canProceed
}

// checkActionConditions checks all types of conditions: Action and Config Conditions
// These are page related conditions, for instance check if an element is present
// or if the page is in the desired language etc.
func checkActionConditions(ctx *processContext, conditions map[string]interface{}, wd *selenium.WebDriver) bool {
	canProceed := true
	// Check the additional conditions
	if len(conditions) > 0 {
		// Check if the page contains a specific element
		if _, ok := conditions["element"]; ok {
			// Check if the element is present
			_, err := (*wd).FindElement(selenium.ByCSSSelector, conditions["element"].(string))
			if err != nil {
				canProceed = false
			}
		}
		// If a language condition is present, check if the page is in the correct language
		if _, ok := conditions["language"]; ok {
			// Get the page language
			lang, err := (*wd).ExecuteScript("return document.documentElement.lang", nil)
			if err != nil {
				canProceed = false
			}
			// Check if the language is correct
			if lang != conditions["language"] {
				canProceed = false
			}
		}
		// If the requested script returns true, proceed
		if _, ok := conditions["plugin_call"]; ok {
			// retrieve the JavaScript from the plugins registry using the value as the key
			plugin, exists := ctx.re.JSPlugins.GetPlugin(conditions["selector"].(string))
			if !exists {
				canProceed = false
			} else {
				pluginCode := plugin.String()
				rval, err := (*wd).ExecuteScript(pluginCode, nil)
				if err != nil {
					canProceed = false
				} else {
					// Process rval
					rvalStr := fmt.Sprintf("%v", rval)
					rvalStr = strings.ToLower(strings.TrimSpace(rvalStr))
					if rvalStr == "true" {
						canProceed = true
					} else {
						canProceed = false
					}
				}
			}
		}
	}
	return canProceed
}

// executePlannedRules executes the rules in the execution plan
func executePlannedRules(wd *selenium.WebDriver, ctx *processContext, planned cfg.ExecutionPlanItem) {
	// Execute the rules in the execution plan
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing planned rules...")
	// Get the rule
	for _, ruleName := range planned.Rules {
		if ruleName == "" {
			continue
		}
		executeActionRuleByName(ruleName, wd, ctx)
	}
}

func executeActionRuleByName(ruleName string, wd *selenium.WebDriver, ctx *processContext) {
	rule, err := ctx.re.GetActionRuleByName(ruleName)
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlError, "getting action rule: %v", err)
		return
	}

	// Execute the rule
	if err = executeActionRule(ctx, rule, wd); err != nil {
		cmn.DebugMsg(cmn.DbgLvlError, "executing action rule: %v", err)
		if !rule.ErrorHandling.Ignore {
			if rule.ErrorHandling.RetryCount > 0 {
				for i := 0; i < rule.ErrorHandling.RetryCount; i++ {
					if rule.ErrorHandling.RetryDelay > 0 {
						time.Sleep(time.Duration(rule.ErrorHandling.RetryDelay) * time.Second)
					}
					if err = executeActionRule(ctx, rule, wd); err == nil {
						break
					}
				}
			}
		}
	}
}

// executePlannedRuleGroups executes the rule groups in the execution plan
func executePlannedRuleGroups(wd *selenium.WebDriver, ctx *processContext, planned cfg.ExecutionPlanItem) {
	// Execute the rule groups in the execution plan
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing planned rule groups...")
	// Get the rule group
	for _, ruleGroupName := range planned.RuleGroups {
		if strings.TrimSpace(ruleGroupName) == "" {
			continue
		}
		rg, err := ctx.re.GetRuleGroupByName(ruleGroupName)
		if err != nil {
			cmn.DebugMsg(cmn.DbgLvlError, "getting rule group '%s': %v", ruleGroupName, err)
		} else {
			// Execute the rule group
			executeActionRules(ctx, rg.GetActionRules(), wd)
			ctx.Status.TotalActions += len(rg.GetActionRules())
		}
	}
}

// executePlannedRulesets executes the rulesets in the execution plan
func executePlannedRulesets(wd *selenium.WebDriver, ctx *processContext, planned cfg.ExecutionPlanItem) {
	// Execute the rulesets in the execution plan
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing planned rulesets...")
	// Get the ruleset
	for _, rulesetName := range planned.Rulesets {
		if rulesetName == "" {
			continue
		}
		rs, err := ctx.re.GetRulesetByName(rulesetName)
		if err != nil {
			cmn.DebugMsg(cmn.DbgLvlError, "getting ruleset: %v", err)
		} else {
			// Execute the ruleset
			executeActionRules(ctx, rs.GetAllEnabledActionRules(), wd)
		}
	}
}
