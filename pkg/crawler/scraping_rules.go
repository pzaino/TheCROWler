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

/////////
// This file is used as a wrapper to the scrapper package, to avoid circular dependencies.
/////////

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cmn "github.com/pzaino/thecrowler/pkg/common"
	cfg "github.com/pzaino/thecrowler/pkg/config"
	rules "github.com/pzaino/thecrowler/pkg/ruleset"
	"golang.org/x/net/html"

	"github.com/tebeka/selenium"
)

const (
	errExecutingScraping = "executing scraping rule: %v"
)

// processScrapingRules processes the scraping rules
func processScrapingRules(wd *selenium.WebDriver, ctx *ProcessContext, url string) (string, error) {
	cmn.DebugMsg(cmn.DbgLvlDebug2, "Starting to search and process CROWler Scraping rules...")

	scrapedDataDoc := ""

	// Run Scraping Rules if any
	if ctx.source.Config != nil {
		// Execute the CROWler rules
		cmn.DebugMsg(cmn.DbgLvlDebug, "Executing CROWler configured Scraping rules...")
		// Execute the rules
		if strings.TrimSpace(string((*ctx.source.Config))) == "{\"config\":\"default\"}" {
			addScrapedDataToDocument(&scrapedDataDoc, runDefaultScrapingRules(wd, ctx))
		} else {
			configStr := string((*ctx.source.Config))
			cmn.DebugMsg(cmn.DbgLvlDebug5, "Configuration: %v", configStr)
		}
	}

	// Check for rules based on the URL
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing CROWler URL-based Scraping rules (if any)...")
	// If the URL matches a rule, execute it
	data, err := executeScrapingRulesByURL(wd, ctx, url)
	addScrapedDataToDocument(&scrapedDataDoc, data)

	return "{" + scrapedDataDoc + "}", err
}

func addScrapedDataToDocument(scrapedDataDoc *string, newScrapedData string) {
	if (*scrapedDataDoc) == "" {
		(*scrapedDataDoc) = newScrapedData
	} else {
		(*scrapedDataDoc) += "," + newScrapedData
	}
}

func executeScrapingRulesByURL(wd *selenium.WebDriver, ctx *ProcessContext, url string) (string, error) {
	scrapedDataDoc := ""

	// Retrieve the rule group by URL
	rg, err := ctx.re.GetRuleGroupByURL(url)
	if err == nil && rg != nil {
		// Execute all the rules in the rule group (the following function also set the Env and clears it)
		var data string
		data, err = executeScrapingRulesInRuleGroup(ctx, rg, wd)
		// Add the data to the document
		data = strings.TrimSpace(data)
		if data != "" && data != "{}" && data != "false" && data != "true" {
			addScrapedDataToDocument(&scrapedDataDoc, data)
		}
	} else {
		cmn.DebugMsg(cmn.DbgLvlDebug, "No rule group found for URL: %v", url)
	}
	if err != nil {
		return scrapedDataDoc, fmt.Errorf("%v", err)
	}

	// Retrieve the ruleset by URL
	rs, err := ctx.re.GetRulesetByURL(url)
	if err == nil && rs != nil {
		// Execute all the rules in the ruleset
		var data string
		data, err = executeScrapingRulesInRuleset(ctx, rs, wd)
		addScrapedDataToDocument(&scrapedDataDoc, data)
	} else {
		cmn.DebugMsg(cmn.DbgLvlDebug, "No ruleset found for URL: %v", url)
	}

	return scrapedDataDoc, err
}

func executeScrapingRulesInRuleset(ctx *ProcessContext, rs *rules.Ruleset, wd *selenium.WebDriver) (string, error) {
	scrapedDataDoc := ""

	// Setup the environment
	rs.SetEnv(ctx.GetContextID())

	for _, r := range rs.GetAllEnabledScrapingRules() {
		// Execute the rule
		cmn.DebugMsg(cmn.DbgLvlDebug3, "Executing rule: %v", r.RuleName)
		scrapedData, err := executeScrapingRule(ctx, &r, wd)
		if err != nil {
			if strings.Contains(err.Error(), "Critical") {
				return "", fmt.Errorf("%v", err)
			}
			cmn.DebugMsg(cmn.DbgLvlError, errExecutingScraping, err)
		}
		addScrapedDataToDocument(&scrapedDataDoc, scrapedData)
	}

	// Reset the environment
	cmn.KVStore.DeleteNonPersistentByCID(ctx.GetContextID())

	return scrapedDataDoc, nil
}

func executeScrapingRulesInRuleGroup(ctx *ProcessContext, rg *rules.RuleGroup, wd *selenium.WebDriver) (string, error) {
	scrapedDataDoc := ""

	// Set the environment
	rg.SetEnv(ctx.GetContextID())

	var err error
	for _, r := range rg.GetScrapingRules() {
		// Execute the rule
		var scrapedData string
		scrapedData, err = executeScrapingRule(ctx, &r, wd)
		addScrapedDataToDocument(&scrapedDataDoc, scrapedData)
	}

	// Reset the environment
	cmn.KVStore.DeleteNonPersistentByCID(ctx.GetContextID())

	return scrapedDataDoc, err
}

// executeScrapingRule executes a single ScrapingRule
func executeScrapingRule(ctx *ProcessContext, r *rules.ScrapingRule,
	wd *selenium.WebDriver) (string, error) {
	var jsonDocument string

	// Execute Wait condition first
	if len(r.WaitConditions) != 0 {
		err := executeWaitConditions(ctx, r.WaitConditions, wd)
		if err != nil {
			return "", fmt.Errorf("executing wait conditions: %v", err)
		}
	}

	// We need to collect errors instead of returning immediately, because we want to
	// execute all the rules in the ruleset and return all the errors at once
	var errList []error

	// Execute the scraping rule
	if shouldExecuteScrapingRule(r, wd) {
		// Apply the rule
		extractedData, err := ApplyRule(ctx, r, wd)
		if err != nil {
			errList = append(errList, err)
		}

		// Process the extracted data
		processedData := processExtractedData(extractedData)
		cleanedData := cleanJSONDocument(processedData)

		jsonData, err := json.Marshal(cleanedData)
		if err != nil {
			errList = append(errList, fmt.Errorf("marshalling JSON: %v", err))
		}
		if len(r.PostProcessing) != 0 {
			runPostProcessingSteps(ctx, &r.PostProcessing, &jsonData)
		}
		rval := strings.TrimSpace(string(jsonData))
		// Remove the leading and trailing {}
		if strings.HasPrefix(rval, "{") && strings.HasSuffix(rval, "}") {
			rval = rval[1:]
			rval = rval[:len(rval)-1]
		}
		jsonDocument = string(rval)
	}

	if len(errList) > 0 {
		// Join all errors
		errStr := ""
		for _, e := range errList {
			errStr += e.Error() + "\n"
		}
		return jsonDocument, fmt.Errorf("executing scraping rule: %v", errStr)
	}

	return jsonDocument, nil
}

func cleanJSONDocument(doc map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for key, value := range doc {
		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively clean nested maps
			cleaned[key] = cleanJSONDocument(v)

		case []interface{}:
			// Filter out unstructured or invalid values in arrays
			var validArray []interface{}
			for _, item := range v {
				switch item := item.(type) {
				case map[string]interface{}:
					validArray = append(validArray, cleanJSONDocument(item))
				case string, float64, bool, nil:
					validArray = append(validArray, item)
				default:
					// Skip unsupported types
					continue
				}
			}
			cleaned[key] = validArray

		case string, float64, bool, nil:
			// Keep valid primitive types
			cleaned[key] = v

		default:
			// Skip unsupported or unstructured types
			continue
		}
	}

	return cleaned
}

func executeWaitConditions(ctx *ProcessContext, conditions []rules.WaitCondition, wd *selenium.WebDriver) error {
	for _, wc := range conditions {
		err := WaitForCondition(ctx, wd, wc)
		if err != nil {
			return fmt.Errorf("executing wait condition: %v", err)
		}
	}
	return nil
}

func shouldExecuteScrapingRule(r *rules.ScrapingRule, wd *selenium.WebDriver) bool {
	return len(r.Conditions) == 0 || checkScrapingConditions(r.Conditions, wd)
}

func processExtractedData(extractedData map[string]interface{}) map[string]interface{} {
	processedData := make(map[string]interface{})

	for key, data := range extractedData {
		if key == "" || key == "false" || key == "true" {
			cmn.DebugMsg(cmn.DbgLvlWarn, "Not allowed key in extracted content: %v", key)
			continue
		}
		switch v := data.(type) {
		case string:
			// Check if the string is valid JSON
			if json.Valid([]byte(v)) {
				var jsonData interface{}
				if err := json.Unmarshal([]byte(v), &jsonData); err == nil {
					processedData[key] = jsonData
					continue
				}
			}

			// Check if the string is HTML
			if StrIsHTML(v) {
				jsonData, err := ProcessHTMLToJSON(v)
				if err == nil {
					processedData[key] = jsonData
				} else {
					processedData[key] = v // Store as raw string
				}
			} else {
				processedData[key] = v // Store as plain string
			}

		case map[string]interface{}:
			// If the data is already a map, store it directly
			processedData[key] = v

		case []interface{}:
			// Process each item in the array
			var processedArray []interface{}
			for _, item := range v {
				switch item := item.(type) {
				case map[string]interface{}:
					processedArray = append(processedArray, processExtractedData(item))
				default:
					// Skip unsupported types
					continue
				}
			}
			processedData[key] = processedArray

		case bool:
			// Handle boolean values and ensure they're keyed
			processedData[key] = v

		default:
			// Fallback for unexpected types
			cmn.DebugMsg(cmn.DbgLvlWarn, "Unexpected type in extracted content: %T", v)
			processedData[key] = v
		}
	}

	return processedData
}

// StrIsHTML checks if the given string could be HTML by trying to parse it.
func StrIsHTML(s string) bool {
	// Minimal check to quickly filter out definitely non-HTML content
	if !strings.Contains(s, "<") && !strings.Contains(s, ">") {
		return false
	}

	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return false // If parsing fails, it's likely not HTML
	}

	var hasElement bool
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data != "html" && n.Data != "head" && n.Data != "body" {
			hasElement = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return hasElement
}

// runPostProcessingSteps runs the post processing steps
func runPostProcessingSteps(ctx *ProcessContext, pps *[]rules.PostProcessingStep, jsonData *[]byte) {
	for _, pp := range *pps {
		// Execute the post processing step
		ApplyPostProcessingStep(ctx, &pp, jsonData)
	}
}

// DefaultCrawlingConfig returns a default configuration for crawling a page
func DefaultCrawlingConfig(url string) cfg.SourceConfig {
	return cfg.SourceConfig{
		FormatVersion: "1.0",
		Author:        "Your Name",
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
					URLPatterns: []string{url},
				},
				Rules: []string{""},
			},
		},
	}
}

func runDefaultScrapingRules(wd *selenium.WebDriver, ctx *ProcessContext) string {
	// Execute the default scraping rules
	cmn.DebugMsg(cmn.DbgLvlDebug, "Executing default scraping rules...")

	// Get the default scraping rules
	url, err := (*wd).CurrentURL()
	if err != nil {
		cmn.DebugMsg(cmn.DbgLvlError, "getting the current URL: %v", err)
		url = ""
	}
	rs := DefaultCrawlingConfig(url)
	// Execute all the rules in the ruleset
	var scrapedDataDoc string
	for _, r := range rs.ExecutionPlan {
		// Check the conditions
		if !checkScrapingPreConditions(r.Conditions, url) {
			continue
		}
		addScrapedDataToDocument(&scrapedDataDoc, executeRulesInExecutionPlan(r, wd, ctx))
	}
	return scrapedDataDoc
}

func executeRulesInExecutionPlan(epi cfg.ExecutionPlanItem, wd *selenium.WebDriver, ctx *ProcessContext) string {
	var scrapedDataDoc string
	// Get the rule
	for _, ruleName := range epi.Rules {
		if ruleName == "" {
			continue
		}
		rule, err := ctx.re.GetScrapingRuleByName(ruleName)
		if err != nil {
			cmn.DebugMsg(cmn.DbgLvlError, "getting scraping rule: %v", err)
		} else {
			// Execute the rule
			scrapedData, err := executeScrapingRule(ctx, rule, wd)
			if err != nil {
				cmn.DebugMsg(cmn.DbgLvlError, errExecutingScraping, err)
			} else {
				scrapedDataDoc += scrapedData
				cmn.DebugMsg(cmn.DbgLvlDebug3, "Scraped data: %v", scrapedDataDoc)
			}
		}
	}
	return scrapedDataDoc
}

// checkScrapingPreConditions checks if the pre conditions are met
// for example if the page URL is listed in the list of URLs
// for which this rule is valid.
func checkScrapingPreConditions(conditions cfg.Condition, url string) bool {
	canProceed := true
	// Check the URL patterns
	if len(conditions.URLPatterns) > 0 {
		for _, pattern := range conditions.URLPatterns {
			if strings.Contains(url, pattern) {
				canProceed = true
			} else {
				canProceed = false
			}
		}
	}
	return canProceed
}

// checkScrapingConditions checks all types of conditions: Scraping and Config Conditions
// These are page related conditions, for instance check if an element is present
// or if the page is in the desired language etc.
func checkScrapingConditions(conditions map[string]interface{}, wd *selenium.WebDriver) bool {
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
	}
	return canProceed
}

func parseHTML(htmlData string) ([]map[string]interface{}, error) {
	doc, err := html.Parse(strings.NewReader(htmlData))
	if err != nil {
		return nil, err
	}
	var items []map[string]interface{}
	parseNode(doc, nil, &items)
	return items, nil
}

func parseNode(n *html.Node, currentItem map[string]interface{}, items *[]map[string]interface{}) {
	if n.Type == html.ElementNode {
		newItem := createNewItem(n)
		if len(newItem) > 0 {
			if currentItem != nil {
				addChild(currentItem, newItem)
			} else {
				addItem(items, newItem)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			parseNode(c, newItem, items)
		}
	} else if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
		if currentItem != nil && len(currentItem) == 0 {
			currentItem["text"] = strings.TrimSpace(n.Data)
		}
	}
}

func createNewItem(n *html.Node) map[string]interface{} {
	newItem := make(map[string]interface{})
	for _, a := range n.Attr {
		newItem[a.Key] = a.Val
	}
	if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
		newItem["text"] = strings.TrimSpace(n.FirstChild.Data)
	}
	return newItem
}

func addChild(currentItem map[string]interface{}, newItem map[string]interface{}) {
	if _, exists := currentItem["children"]; !exists {
		currentItem["children"] = []map[string]interface{}{}
	}
	currentItem["children"] = append(currentItem["children"].([]map[string]interface{}), newItem)
}

func addItem(items *[]map[string]interface{}, newItem map[string]interface{}) {
	*items = append(*items, newItem)
}

func toJSON(data []map[string]interface{}) (string, error) {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// ProcessHTMLToJSON processes the HTML data and converts it to JSON
func ProcessHTMLToJSON(htmlData string) (string, error) {
	items, err := parseHTML(htmlData)
	if err != nil {
		return "", err
	}
	jsonOutput, err := toJSON(items)
	if err != nil {
		return "", err
	}
	return jsonOutput, nil
}
