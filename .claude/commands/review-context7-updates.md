<INSTRUCTIONS note="Do not modify this section">
Please read the @README.md and @docs/tools/package-documentation.md to understand the high level context of my repo.

My package documentation tool provides context7 functionality so you don't have to use the official context7 mcp server as well.

There were some updates to the official context7 mcp server since I last looked and it and I'd like to task you with determining if any new features or improvements were added to it that
  we should consider replicating in our package-documentation tool.

I've clone the official context7 server to $HOME/git/mcp/context7, you may need to do a git pull of that repo to get the latest changes.

Please first review my package documentation tool to understand its capabilities, then review the changes in context7 to see if there are any gaps. If there are gaps or improvements we should look at please provide a summary and list of them here for me to consider, if we should implement them, and if so how we should do it ensuring we align with my projects existing code, patterns and configuration.

</INSTRUCTIONS>

---

<EXAMPLE_SUMMARY note="Do not modify this section">

Changes since `479473a` to `784ef42` (current) in context7:

- Proxy Support, new proxy support was added to context7 allowing it to work in enterprise environments
- Trust Score Prioritisation, context7 now allows prioritising sources based on trust scores using github stars and other metrics
- Client IP Encryption, context7 added support for encrypting client IPs for privacy
- HTTP Transport, context7 mcp server added support for HTTP transport in addition to STDIO
- Auth Headers, context7 added support for more complex authentication headers for various auth schemes

Recommendation

Implement:
1. ✅ Proxy Support - Aligns with your enterprise-focused architecture
2. ✅ Trust Score Prioritisation - Simple text changes
3. ✅ Source Header - One line addition

Skip:
1. ❌ Client IP Encryption - Adds complexity without clear benefit for your use case
2. ❌ HTTP Transport - Your tool focuses on STDIO MCP integration
3. ❌ Complex Auth Headers - Your current auth handling is sufficient

Implementation Steps (if approved):
- [ ] Add proxy configuration options to <files>, ensuring we follow existing config patterns, e.g. <existing patterns to consider>
- [ ] ....

</EXAMPLE_SUMMARY>

---

## Versions

As part of your report ensure you record the git sha of the context7 repo of the last time we reviewed it and the current git sha so we can track what has changed.

IMPORTANT: Update these git shas in the `.claude/commands/review-context7-updates.md` file after your review.

- Previously reviewed context7 git sha range (before this review): `479473a` to `784ef42`
- Next context7 git sha range (this review):

Historical reviews:
- Review Date - from `479473a` to `784ef42`
- ...
