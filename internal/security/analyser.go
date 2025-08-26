package security

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
)

// AnalyseContent performs Intent-Context-Destination analysis on content
func (a *SecurityAdvisor) AnalyseContent(content string, source SourceContext) (*SecurityResult, error) {
	// Use cache if enabled
	if a.config.CacheEnabled {
		return a.cache.GetWithGeneration(content, source, func() (*SecurityResult, error) {
			return a.performAnalysis(content, source)
		})
	}

	return a.performAnalysis(content, source)
}

// performAnalysis performs the actual security analysis
func (a *SecurityAdvisor) performAnalysis(content string, source SourceContext) (*SecurityResult, error) {
	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"content_length":         len(content),
			"source_domain":          source.Domain,
			"max_scan_size":          a.config.MaxScanSize,
			"threat_threshold":       a.config.ThreatThreshold,
			"enable_base64_scanning": a.config.EnableBase64Scanning,
		}).Debug("Beginning performAnalysis on content")
	}

	// Skip analysis for obviously safe content types
	if !a.shouldAnalyseContent(content, source) {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"content_length": len(content),
				"source_domain":  source.Domain,
				"content_type":   source.ContentType,
			}).Debug("Content skipped as obviously safe")
		}
		return &SecurityResult{Safe: true, Action: ActionAllow}, nil
	}

	originalContentLength := len(content)
	// Check size limits
	if len(content) > a.config.MaxScanSize {
		// For large content, only scan the first portion
		content = content[:a.config.MaxScanSize]
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"original_length": originalContentLength,
				"truncated_to":    len(content),
				"max_scan_size":   a.config.MaxScanSize,
			}).Debug("Content truncated due to size limits")
		}
	}

	// Apply encoding detection and normalisation to prevent pattern evasion
	processedContent := a.applyEncodingDetection(content)
	contentWasModified := processedContent != content

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"original_content_length":  len(content),
			"processed_content_length": len(processedContent),
			"content_was_modified":     contentWasModified,
			"processed_preview":        processedContent[:min(100, len(processedContent))],
		}).Debug("Content encoding detection and processing completed")
	}

	// Perform threat analysis on both original and processed content
	analysis := a.analyser.AnalyseContent(content, source, a.ruleEngine)

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"original_commands_detected":  len(analysis.Commands),
			"original_risk_factors_count": len(analysis.RiskFactors),
			"original_risk_factors":       analysis.RiskFactors,
		}).Debug("Original content threat analysis completed")
	}

	// If original content was clean, also check processed content
	if len(analysis.Commands) == 0 && len(analysis.RiskFactors) == 0 && contentWasModified {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.Debug("Original content clean, analysing processed content for encoded threats")
		}

		processedAnalysis := a.analyser.AnalyseContent(processedContent, source, a.ruleEngine)
		if len(processedAnalysis.Commands) > 0 || len(processedAnalysis.RiskFactors) > 0 {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.WithFields(logrus.Fields{
					"processed_commands_detected": len(processedAnalysis.Commands),
					"processed_risk_factors":      processedAnalysis.RiskFactors,
					"original_was_clean":          true,
				}).Debug("Encoded content evasion detected - threats found in processed content")
			}

			// Found threats in processed content - merge results
			analysis.Commands = append(analysis.Commands, processedAnalysis.Commands...)
			analysis.RiskFactors = append(analysis.RiskFactors, processedAnalysis.RiskFactors...)
			analysis.RiskFactors = append(analysis.RiskFactors, "encoded content evasion")
		} else {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.Debug("Processed content also clean - no encoded threats detected")
			}
		}
	}

	// Get source trust score
	analysis.SourceTrust = a.trust.GetTrustScore(source.Domain)
	analysis.Context = a.categoriseSource(source)

	// Calculate overall risk
	analysis.RiskScore = a.calculateRisk(analysis)

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"source_trust":       analysis.SourceTrust,
			"source_context":     analysis.Context,
			"calculated_risk":    analysis.RiskScore,
			"threat_threshold":   a.config.ThreatThreshold,
			"total_commands":     len(analysis.Commands),
			"total_risk_factors": len(analysis.RiskFactors),
		}).Debug("Risk calculation completed")
	}

	// Use rule engine for pattern-based analysis on original content
	ruleResult, err := a.ruleEngine.EvaluateContentWithConfig(content, source, a.config)
	if err != nil {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithError(err).Debug("Rule engine evaluation failed, proceeding with basic analysis")
		}
		// Log error but continue with basic analysis
		return &SecurityResult{
			Safe:      analysis.RiskScore < a.config.ThreatThreshold,
			Action:    a.determineAction(analysis.RiskScore),
			Message:   a.formatAnalysisMessage(analysis),
			Analysis:  analysis,
			Timestamp: ruleResult.Timestamp,
		}, nil
	}

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"rule_result_safe":    ruleResult.Safe,
			"rule_result_action":  ruleResult.Action,
			"rule_result_message": ruleResult.Message,
		}).Debug("Rule engine evaluation of original content completed")
	}

	// If original content passed but we have processed content, check that too
	if ruleResult.Safe && contentWasModified {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.Debug("Original content passed rules, evaluating processed content")
		}

		processedRuleResult, err := a.ruleEngine.EvaluateContentWithConfig(processedContent, source, a.config)
		if err == nil && !processedRuleResult.Safe {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.WithFields(logrus.Fields{
					"processed_rule_safe":    processedRuleResult.Safe,
					"processed_rule_action":  processedRuleResult.Action,
					"processed_rule_message": processedRuleResult.Message,
				}).Debug("Processed content failed rules - encoded content evasion detected")
			}

			// Processed content triggered rules - this indicates encoding evasion
			processedRuleResult.Analysis = analysis
			processedRuleResult.Message = "Encoded content evasion detected: " + processedRuleResult.Message
			return processedRuleResult, nil
		} else if err == nil {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.Debug("Processed content also passed rules")
			}
		}
	}

	// Combine rule-based and analysis-based results
	if !ruleResult.Safe {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithField("rule_triggered", true).Debug("Rule engine found specific threats")
		}
		// Rule engine found specific threats
		ruleResult.Analysis = analysis
		return ruleResult, nil
	}

	// Return analysis-based result
	finalResult := &SecurityResult{
		Safe:      analysis.RiskScore < a.config.ThreatThreshold,
		Action:    a.determineAction(analysis.RiskScore),
		Message:   a.formatAnalysisMessage(analysis),
		Analysis:  analysis,
		Timestamp: ruleResult.Timestamp,
	}

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"final_safe":    finalResult.Safe,
			"final_action":  finalResult.Action,
			"final_message": finalResult.Message,
		}).Debug("Security analysis completed successfully")
	}

	return finalResult, nil
}

// AnalyseContent performs threat analysis on content
func (t *ThreatAnalyser) AnalyseContent(content string, source SourceContext, ruleEngine *YAMLRuleEngine) *ThreatAnalysis {
	analysis := &ThreatAnalysis{
		Commands:    t.parseCommands(content, ruleEngine),
		RiskFactors: []string{},
	}

	return analysis
}

// parseCommands detects and parses shell commands in content using rule engine
func (t *ThreatAnalyser) parseCommands(content string, ruleEngine *YAMLRuleEngine) []ParsedCommand {
	var commands []ParsedCommand

	// Use rule engine to find shell injection patterns
	if ruleEngine != nil && ruleEngine.rules != nil {
		// Check shell_injection rule specifically for command parsing
		if shellRule, exists := ruleEngine.rules.Rules["shell_injection"]; exists {
			// Extract all regex matches from the shell_injection rule
			for i, pattern := range shellRule.Patterns {
				if pattern.Regex != "" {
					key := fmt.Sprintf("shell_injection_%d", i)
					if matcher, exists := ruleEngine.compiled[key]; exists {
						if regexMatcher, ok := matcher.(*RegexMatcher); ok {
							patternMatches := regexMatcher.regex.FindAllString(content, -1)
							for _, match := range patternMatches {
								cmd := t.parseCommand(match)
								if cmd != nil {
									commands = append(commands, *cmd)
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback: simple pattern detection if rule engine isn't available
	if len(commands) == 0 {
		// Look for basic command patterns as fallback
		simplePatterns := []string{
			"curl", "wget", "eval", "exec", "$(",
		}

		for _, pattern := range simplePatterns {
			if strings.Contains(content, pattern) {
				// Create a basic parsed command for simple detection
				cmd := &ParsedCommand{
					Raw:        pattern,
					Executable: pattern,
					Arguments:  []CommandArgument{},
				}
				commands = append(commands, *cmd)
			}
		}
	}

	return commands
}

// parseCommand parses a single command string
func (t *ThreatAnalyser) parseCommand(cmdStr string) *ParsedCommand {
	// Clean the command string
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}

	// Try to parse with shlex
	parts, err := shlex.Split(cmdStr)
	if err != nil {
		// Fall back to simple space splitting
		parts = strings.Fields(cmdStr)
	}

	if len(parts) == 0 {
		return nil
	}

	cmd := &ParsedCommand{
		Raw:        cmdStr,
		Executable: parts[0],
		Arguments:  []CommandArgument{},
	}

	// Parse arguments
	for i := 1; i < len(parts); i++ {
		arg := t.analyseArgument(parts[i])
		cmd.Arguments = append(cmd.Arguments, arg)
	}

	// Detect pipe operations
	if strings.Contains(cmdStr, "|") {
		cmd.Pipes = t.parsePipeOperations(cmdStr)
	}

	// Analyse destination if it's a network command
	if t.isNetworkCommand(cmd.Executable) {
		cmd.Destination = t.extractDestination(cmd.Arguments)
	}

	return cmd
}

// analyseArgument analyses a command argument for security properties
func (t *ThreatAnalyser) analyseArgument(arg string) CommandArgument {
	cmdArg := CommandArgument{
		Value:      arg,
		Type:       t.determineArgumentType(arg),
		TrustScore: 1.0, // Default trust
	}

	// Calculate entropy
	cmdArg.EntropyScore = t.calculateEntropy(arg)

	// Check for variable substitution
	cmdArg.IsVariable = strings.Contains(arg, "$") ||
		strings.Contains(arg, "${") ||
		strings.Contains(arg, "$(")

	// Check for potential secrets
	cmdArg.ContainsSecrets = t.looksLikeSecret(arg)

	// Adjust trust score based on characteristics
	if cmdArg.IsVariable {
		cmdArg.TrustScore -= 0.3
	}
	if cmdArg.EntropyScore > 6.0 {
		cmdArg.TrustScore -= 0.4
	}
	if cmdArg.ContainsSecrets {
		cmdArg.TrustScore -= 0.5
	}

	return cmdArg
}

// determineArgumentType determines the type of command argument
func (t *ThreatAnalyser) determineArgumentType(arg string) ArgumentType {
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return ArgumentTypeURL
	}
	if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "~/") {
		return ArgumentTypeFile
	}
	if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return ArgumentTypeFlag
	}
	if strings.Contains(arg, "$") {
		return ArgumentTypeVariable
	}
	return ArgumentTypeString
}

// parsePipeOperations parses shell pipe operations
func (t *ThreatAnalyser) parsePipeOperations(cmdStr string) []PipeOperation {
	var pipes []PipeOperation

	parts := strings.Split(cmdStr, "|")
	for i := 0; i < len(parts)-1; i++ {
		source := strings.TrimSpace(parts[i])
		target := strings.TrimSpace(parts[i+1])

		pipe := PipeOperation{
			Source:      source,
			Target:      target,
			IsShell:     t.isShellCommand(target),
			IsDangerous: t.isDangerousPipe(source, target),
		}

		pipes = append(pipes, pipe)
	}

	return pipes
}

// isNetworkCommand checks if a command makes network requests
func (t *ThreatAnalyser) isNetworkCommand(cmd string) bool {
	networkCommands := []string{"curl", "wget", "fetch", "nc", "netcat", "telnet", "ssh", "scp", "rsync"}
	cmd = strings.ToLower(cmd)
	return slices.Contains(networkCommands, cmd)
}

// isShellCommand checks if a command is a shell interpreter
func (t *ThreatAnalyser) isShellCommand(cmd string) bool {
	shellCommands := []string{"sh", "bash", "zsh", "fish", "csh", "tcsh", "ksh"}
	cmd = strings.ToLower(strings.Fields(cmd)[0])
	return slices.Contains(shellCommands, cmd)
}

// isDangerousPipe checks if a pipe operation is potentially dangerous
func (t *ThreatAnalyser) isDangerousPipe(source, target string) bool {
	// Network command piping to shell is dangerous
	sourceParts := strings.Fields(source)
	if len(sourceParts) > 0 && t.isNetworkCommand(sourceParts[0]) && t.isShellCommand(target) {
		return true
	}

	// Piping encoded content to shell
	if strings.Contains(source, "base64") && t.isShellCommand(target) {
		return true
	}

	return false
}

// extractDestination extracts destination information from command arguments
func (t *ThreatAnalyser) extractDestination(args []CommandArgument) *Destination {
	for _, arg := range args {
		if arg.Type == ArgumentTypeURL {
			return t.analyseDestination(arg.Value)
		}
	}
	return nil
}

// analyseDestination analyses a destination URL for trust and reputation
func (t *ThreatAnalyser) analyseDestination(urlStr string) *Destination {
	// Simple URL parsing for host extraction
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return nil
	}

	// Extract host portion
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")
	host := strings.Split(urlStr, "/")[0]
	host = strings.Split(host, ":")[0] // Remove port

	destination := &Destination{
		URL:             urlStr,
		Host:            host,
		ReputationScore: t.calculateReputationScore(host),
		Category:        t.categoriseDestination(host),
	}

	return destination
}

// calculateEntropy calculates the Shannon entropy of a string
func (t *ThreatAnalyser) calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count character frequencies
	freq := make(map[rune]float64)
	for _, char := range s {
		freq[char]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	length := float64(len(s))
	for _, count := range freq {
		probability := count / length
		entropy -= probability * math.Log2(probability)
	}

	return entropy
}

// looksLikeSecret checks if a string looks like a secret or credential
func (t *ThreatAnalyser) looksLikeSecret(s string) bool {
	// High entropy strings that might be secrets
	if len(s) > 20 && t.calculateEntropy(s) > 6.0 {
		return true
	}

	// Common secret patterns
	secretPatterns := []string{
		"api_key", "apikey", "secret", "token", "password", "passwd",
		"private_key", "aws_access", "aws_secret",
	}

	lower := strings.ToLower(s)
	for _, pattern := range secretPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// calculateReputationScore calculates a reputation score for a host
func (t *ThreatAnalyser) calculateReputationScore(host string) float64 {
	// Simple reputation scoring based on common patterns
	score := 0.5 // Neutral

	// Known good domains get higher scores
	goodDomains := []string{"github.com", "gitlab.com", "docker.com", "kubernetes.io", "golang.org"}
	for _, domain := range goodDomains {
		if strings.Contains(host, domain) {
			score = 0.9
			break
		}
	}

	// Suspicious patterns reduce score
	if strings.Contains(host, "bit.ly") || strings.Contains(host, "tinyurl") {
		score -= 0.3
	}

	// IP addresses are less trustworthy
	if regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`).MatchString(host) {
		score -= 0.2
	}

	return math.Max(0, math.Min(1, score))
}

// categoriseDestination categorises a destination host
func (t *ThreatAnalyser) categoriseDestination(host string) DestinationCategory {
	// Official domains
	officialDomains := []string{"github.com", "gitlab.com", "docker.com", "kubernetes.io"}
	for _, domain := range officialDomains {
		if strings.Contains(host, domain) {
			return DestinationOfficial
		}
	}

	// CDN domains
	cdnDomains := []string{"amazonaws.com", "cloudfront.net", "googleapis.com"}
	for _, domain := range cdnDomains {
		if strings.Contains(host, domain) {
			return DestinationCDN
		}
	}

	// Suspicious patterns
	if strings.Contains(host, "bit.ly") || strings.Contains(host, "tinyurl") {
		return DestinationSuspicious
	}

	return DestinationUnknown
}

// GetTrustScore returns a trust score for a domain
func (s *SourceTrust) GetTrustScore(domain string) float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	domain = strings.ToLower(domain)

	// Check trusted domains
	for _, trusted := range s.trustedDomains {
		if s.domainMatches(domain, trusted) {
			return 1.0
		}
	}

	// Check suspicious domains
	for _, suspicious := range s.suspiciousDomains {
		if s.domainMatches(domain, suspicious) {
			return 0.0
		}
	}

	// Default neutral trust
	return 0.5
}

// domainMatches checks if domain matches pattern (supports wildcards)
func (s *SourceTrust) domainMatches(domain, pattern string) bool {
	if after, ok := strings.CutPrefix(pattern, "*."); ok {
		baseDomain := after
		return domain == baseDomain || strings.HasSuffix(domain, "."+baseDomain)
	}
	return domain == pattern
}

// shouldAnalyseContent determines if content should be analysed
func (a *SecurityAdvisor) shouldAnalyseContent(content string, source SourceContext) bool {
	// Skip very short content
	if len(content) < 50 {
		return false
	}

	// Skip obviously safe content types
	if strings.HasPrefix(source.ContentType, "image/") ||
		strings.HasPrefix(source.ContentType, "video/") ||
		strings.HasPrefix(source.ContentType, "audio/") {
		return false
	}

	// Always analyse if content contains suspicious patterns
	suspiciousPatterns := []string{"curl", "wget", "eval", "exec", "$", "|", "base64"}
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return true
}

// categoriseSource categorises the source context
func (a *SecurityAdvisor) categoriseSource(source SourceContext) string {
	domain := strings.ToLower(source.Domain)

	// Official documentation domains
	officialDocs := []string{"docs.docker.com", "kubernetes.io/docs", "golang.org/doc"}
	for _, doc := range officialDocs {
		if strings.Contains(domain, doc) {
			return "official_docs"
		}
	}

	// Community/GitHub pages
	if strings.Contains(domain, "github.io") || strings.Contains(domain, "github.com") {
		return "community"
	}

	// CDN/Infrastructure
	if strings.Contains(domain, "amazonaws.com") || strings.Contains(domain, "cloudfront.net") {
		return "cdn"
	}

	return "unknown"
}

// calculateRisk calculates overall risk score based on analysis
func (a *SecurityAdvisor) calculateRisk(analysis *ThreatAnalysis) float64 {
	risk := 0.0

	// Command risk assessment
	for _, cmd := range analysis.Commands {
		if a.hasVariableSubstitution(cmd.Raw) {
			risk += 0.3
			analysis.RiskFactors = append(analysis.RiskFactors, "variable substitution")
		}

		if a.hasEncoding(cmd.Raw) {
			risk += 0.4
			analysis.RiskFactors = append(analysis.RiskFactors, "encoded content")
		}

		if a.hasDestructivePatterns(cmd.Raw) {
			risk += 0.6
			analysis.RiskFactors = append(analysis.RiskFactors, "destructive command")
		}

		// Check pipe operations
		for _, pipe := range cmd.Pipes {
			if pipe.IsDangerous {
				risk += 0.5
				analysis.RiskFactors = append(analysis.RiskFactors, "dangerous pipe operation")
			}
		}
	}

	// Adjust risk based on source trust
	risk = risk * (1.0 - analysis.SourceTrust)

	// Context adjustments
	if analysis.Context == "official_docs" {
		risk *= 0.3 // Reduce risk for official documentation
	}

	return math.Min(1.0, risk)
}

// hasVariableSubstitution checks for variable substitution patterns
func (a *SecurityAdvisor) hasVariableSubstitution(content string) bool {
	patterns := []string{"${", "$(", "`"}
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// hasEncoding checks for encoded content
func (a *SecurityAdvisor) hasEncoding(content string) bool {
	// Base64 patterns
	base64Pattern := regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}`)
	return base64Pattern.MatchString(content)
}

// hasDestructivePatterns checks for obviously destructive commands
func (a *SecurityAdvisor) hasDestructivePatterns(content string) bool {
	// Use rule engine to check for obvious_malware patterns
	if a.ruleEngine != nil && a.ruleEngine.rules != nil {
		if rule, exists := a.ruleEngine.rules.Rules["obvious_malware"]; exists {
			// Use the evaluateRuleWithConfig method to enable base64 processing
			matches, _ := a.ruleEngine.evaluateRuleWithConfig("obvious_malware", rule, content, SourceContext{}, a.config)
			return matches
		}
	}

	// Fallback: basic literal checks if rule engine not available
	destructivePatterns := []string{
		"rm -rf /",
		"rm -rf /*",
		":(){ :|:& };:",
		"dd if=/dev/zero",
	}

	for _, pattern := range destructivePatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// determineAction determines the appropriate action based on risk score
func (a *SecurityAdvisor) determineAction(riskScore float64) string {
	if riskScore >= 0.8 {
		return ActionBlock
	} else if riskScore >= 0.4 {
		return ActionWarn
	}
	return ActionAllow
}

// formatAnalysisMessage formats a user-friendly analysis message
func (a *SecurityAdvisor) formatAnalysisMessage(analysis *ThreatAnalysis) string {
	if len(analysis.RiskFactors) == 0 {
		return "Content appears safe"
	}

	factors := strings.Join(analysis.RiskFactors, ", ")
	return fmt.Sprintf("Security concerns detected: %s", factors)
}

// applyEncodingDetection applies encoding detection and normalisation to prevent pattern evasion
func (a *SecurityAdvisor) applyEncodingDetection(content string) string {
	// Start with Unicode normalisation
	normalized := a.normalizeUnicode(content)

	// Apply base64 detection and decoding
	decoded := a.detectAndDecodeBase64(normalized)

	// Apply URL decoding for common URL encoding evasion
	urlDecoded := a.decodeURLEncoding(decoded)

	// Apply hex decoding for hex-encoded content
	hexDecoded := a.decodeHexEncoding(urlDecoded)

	return hexDecoded
}

// normalizeUnicode normalizes Unicode content to prevent evasion through different Unicode forms
func (a *SecurityAdvisor) normalizeUnicode(content string) string {
	// Apply NFC (Canonical Decomposition followed by Canonical Composition)
	// This converts visually identical Unicode characters to the same representation
	normalized := norm.NFC.String(content)

	// Remove/replace invisible and confusing Unicode characters
	var result strings.Builder
	for _, r := range normalized {
		switch {
		case r == '\u200B': // Zero-width space
			// Skip zero-width spaces as they can hide malicious content
			continue
		case r == '\u200C' || r == '\u200D': // Zero-width non-joiner/joiner
			// Skip these as well
			continue
		case r == '\uFEFF': // Byte order mark
			// Skip BOM characters
			continue
		case unicode.Is(unicode.Cf, r): // Format characters
			// Skip most format characters that could be used for evasion
			continue
		case !utf8.ValidRune(r):
			// Replace invalid runes with replacement character
			result.WriteRune('\uFFFD')
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// detectAndDecodeBase64 detects and decodes base64 content to reveal hidden commands
func (a *SecurityAdvisor) detectAndDecodeBase64(content string) string {
	// Skip if base64 scanning is disabled
	if !a.config.EnableBase64Scanning {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithField("content_length", len(content)).Debug("Base64 scanning disabled, skipping detection")
		}
		return content
	}

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"content_length":         len(content),
			"max_base64_size_kb":     a.config.MaxBase64DecodedSize,
			"enable_base64_scanning": a.config.EnableBase64Scanning,
		}).Debug("Starting base64 detection and decoding")
	}

	// Use our safe heuristic detection first
	lines := strings.Split(content, "\n")
	var resultLines []string
	totalLinesProcessed := 0
	base64LinesDetected := 0
	base64LinesDecoded := 0

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		totalLinesProcessed++

		// Skip empty lines or lines that don't look like base64
		if len(line) == 0 {
			resultLines = append(resultLines, line)
			continue
		}

		isBase64Candidate := isLikelyBase64(line)
		if logrus.GetLevel() <= logrus.DebugLevel && len(line) > 16 {
			logrus.WithFields(logrus.Fields{
				"line_number":         lineNum + 1,
				"line_length":         len(line),
				"line_preview":        line[:min(50, len(line))],
				"is_base64_candidate": isBase64Candidate,
			}).Debug("Evaluating line for base64 detection")
		}

		if !isBase64Candidate {
			resultLines = append(resultLines, line)
			continue
		}

		base64LinesDetected++

		// Use safe decoding with configured size limits
		maxSize := a.config.MaxBase64DecodedSize * 1024 // Convert KB to bytes
		decoded, success := safeBase64Decode(line, maxSize)

		if !success {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.WithFields(logrus.Fields{
					"line_number":  lineNum + 1,
					"line_length":  len(line),
					"line_preview": line[:min(50, len(line))],
					"max_size_kb":  a.config.MaxBase64DecodedSize,
				}).Debug("Base64 decoding failed or size limit exceeded")
			}
			// Decoding failed or size limit exceeded, keep original
			resultLines = append(resultLines, line)
			continue
		}

		base64LinesDecoded++

		// Check if decoded content is printable text
		decodedStr := string(decoded)
		isPrintable := a.isPrintableText(decodedStr)

		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"line_number":      lineNum + 1,
				"original_length":  len(line),
				"decoded_length":   len(decodedStr),
				"decoded_preview":  decodedStr[:min(50, len(decodedStr))],
				"is_printable":     isPrintable,
				"original_preview": line[:min(50, len(line))],
			}).Debug("Base64 content successfully decoded")
		}

		if isPrintable {
			// Return both original and decoded for analysis
			resultLines = append(resultLines, line+" "+decodedStr)
		} else {
			// Non-printable decoded content, keep original only
			resultLines = append(resultLines, line)
		}
	}

	resultContent := strings.Join(resultLines, "\n")

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"total_lines_processed":    totalLinesProcessed,
			"base64_lines_detected":    base64LinesDetected,
			"base64_lines_decoded":     base64LinesDecoded,
			"original_content_length":  len(content),
			"processed_content_length": len(resultContent),
			"content_modified":         len(resultContent) != len(content),
		}).Debug("Base64 detection and decoding completed")
	}

	return resultContent
}

// decodeURLEncoding decodes URL-encoded content
func (a *SecurityAdvisor) decodeURLEncoding(content string) string {
	// Pattern for URL encoding (%XX format)
	urlPattern := regexp.MustCompile(`%[0-9A-Fa-f]{2}`)

	if !urlPattern.MatchString(content) {
		return content // No URL encoding found
	}

	result := urlPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Decode the hex value
		if len(match) == 3 && match[0] == '%' {
			hex := match[1:3]
			if val, err := parseHexByte(hex); err == nil {
				return string(rune(val))
			}
		}
		return match
	})

	return result
}

// decodeHexEncoding decodes hex-encoded content (0x format or plain hex)
func (a *SecurityAdvisor) decodeHexEncoding(content string) string {
	// Pattern for hex encoding (0x followed by hex digits, or \x format)
	hexPattern := regexp.MustCompile(`(?:0x|\\x)([0-9A-Fa-f]{2})`)

	result := hexPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the hex part
		var hex string
		if strings.HasPrefix(match, "0x") {
			hex = match[2:]
		} else if strings.HasPrefix(match, "\\x") {
			hex = match[2:]
		} else {
			return match
		}

		if val, err := parseHexByte(hex); err == nil {
			return string(rune(val))
		}
		return match
	})

	return result
}

// isPrintableText checks if decoded content consists mainly of printable text
func (a *SecurityAdvisor) isPrintableText(s string) bool {
	if len(s) == 0 {
		return false
	}

	printableCount := 0
	for _, r := range s {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printableCount++
		}
	}

	// Consider it printable if at least 80% of characters are printable
	return float64(printableCount)/float64(utf8.RuneCountInString(s)) >= 0.8
}

// parseHexByte parses a 2-character hex string into a byte value
func parseHexByte(hex string) (byte, error) {
	if len(hex) != 2 {
		return 0, fmt.Errorf("invalid hex length")
	}

	var result byte
	for _, char := range hex {
		var val byte
		switch {
		case char >= '0' && char <= '9':
			val = byte(char - '0')
		case char >= 'A' && char <= 'F':
			val = byte(char - 'A' + 10)
		case char >= 'a' && char <= 'f':
			val = byte(char - 'a' + 10)
		default:
			return 0, fmt.Errorf("invalid hex character: %c", char)
		}
		result = result*16 + val
	}

	return result, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
