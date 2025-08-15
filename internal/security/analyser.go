package security

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/google/shlex"
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
	// Skip analysis for obviously safe content types
	if !a.shouldAnalyseContent(content, source) {
		return &SecurityResult{Safe: true, Action: ActionAllow}, nil
	}

	// Check size limits
	if len(content) > a.config.MaxScanSize {
		// For large content, only scan the first portion
		content = content[:a.config.MaxScanSize]
	}

	// Perform threat analysis
	analysis := a.analyser.AnalyseContent(content, source, a.ruleEngine)

	// Get source trust score
	analysis.SourceTrust = a.trust.GetTrustScore(source.Domain)
	analysis.Context = a.categoriseSource(source)

	// Calculate overall risk
	analysis.RiskScore = a.calculateRisk(analysis)

	// Use rule engine for pattern-based analysis
	ruleResult, err := a.ruleEngine.EvaluateContent(content, source)
	if err != nil {
		// Log error but continue with basic analysis
		return &SecurityResult{
			Safe:      analysis.RiskScore < a.config.ThreatThreshold,
			Action:    a.determineAction(analysis.RiskScore),
			Message:   a.formatAnalysisMessage(analysis),
			Analysis:  analysis,
			Timestamp: ruleResult.Timestamp,
		}, nil
	}

	// Combine rule-based and analysis-based results
	if !ruleResult.Safe {
		// Rule engine found specific threats
		ruleResult.Analysis = analysis
		return ruleResult, nil
	}

	// Return analysis-based result
	return &SecurityResult{
		Safe:      analysis.RiskScore < a.config.ThreatThreshold,
		Action:    a.determineAction(analysis.RiskScore),
		Message:   a.formatAnalysisMessage(analysis),
		Analysis:  analysis,
		Timestamp: ruleResult.Timestamp,
	}, nil
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
	for _, netCmd := range networkCommands {
		if cmd == netCmd {
			return true
		}
	}
	return false
}

// isShellCommand checks if a command is a shell interpreter
func (t *ThreatAnalyser) isShellCommand(cmd string) bool {
	shellCommands := []string{"sh", "bash", "zsh", "fish", "csh", "tcsh", "ksh"}
	cmd = strings.ToLower(strings.Fields(cmd)[0])
	for _, shellCmd := range shellCommands {
		if cmd == shellCmd {
			return true
		}
	}
	return false
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
	if strings.HasPrefix(pattern, "*.") {
		baseDomain := strings.TrimPrefix(pattern, "*.")
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
			// Use the private evaluateRule method
			matches, _ := a.ruleEngine.evaluateRule("obvious_malware", rule, content, SourceContext{})
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
