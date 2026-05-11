# Prompt for sanitizing HTML research files

Copy and paste this to another Claude Code session pointed at your polybolt-research repo:

---

I need you to sanitize the HTML research report files in this repo for use as public demo content. The files are in `results/studies/` and `research/`. These are real quant research reports that I want to use as demo screenshots for an open-source tool, but I need all sensitive/identifying information changed while keeping the reports looking realistic and professional.

## What to change:

### 1. Asset/Market references
- Replace all references to "BTC" with "SPY" (S&P 500 ETF — a boring, public, non-controversial asset)
- Replace "binary options" with "volatility forecasting" or "directional prediction"
- Replace any crypto-specific terminology with equity/ETF equivalents
- If there are references to specific exchanges, replace with generic terms like "primary exchange"

### 2. Dollar amounts and PnL
- All dollar amounts should be scaled to look like small research account numbers (under $50k total)
- Keep the relative direction the same (losses stay losses, gains stay gains)
- Replace specific values like "$-1,070,005.80" with something like "$-4,231.15"
- Replace "$-97,540.37" with something like "$-412.88"
- Keep numbers looking natural with cents, not round numbers

### 3. Fill counts and market counts
- Scale down fill counts by ~100x (10,026,742 fills → ~100,267 fills)
- Keep market/window counts reasonable (3429 mkts → ~340 mkts)

### 4. Research-specific content
- Keep the general research narrative and conclusions intact — the findings should still make sense
- kNN model references are fine to keep (it's a common ML technique, not proprietary)
- Keep statistical measures like BSS, Win Rate, pp (percentage points)
- Change "polybolt" or any project/company name references to something generic like "research" or "quant-lab"
- Remove any author names, email addresses, or personal identifiers

### 5. Dates
- Keep dates as-is (2026 dates are fine)

### 6. What NOT to change:
- Don't modify the HTML structure, CSS, themes, or JavaScript
- Don't touch embedded base64 images/charts — they contain plot data but no identifying text visible in the charts themselves
- Keep the Light/Dark/Solarized theme switcher
- Keep all interactive elements working

## Files to modify:
- `results/studies/knn_interpolation_posthoc_1h.html`
- `results/studies/knn_theta_resolution.html`
- `results/studies/knn_interpolation_comparison_1h.html`
- `results/studies/knn_subminute_theta_analysis.html`
- `results/studies/knn_subminute_theta_empirical.html`
- `results/studies/knn_subminute_theta_analysis_5m.html`
- `research/cond_prob_backtest_review/analysis_1h.html`
- `research/cond_prob_backtest_review/analysis_5m.html`

## Important:
- Work through each file one by one
- Only modify text content, not HTML/CSS/JS structure
- After all changes, the reports should look like a small independent quant researcher studying equity/ETF volatility prediction using kNN — completely detached from crypto, any specific firm, or any real trading operation
- The research should still tell a coherent story and the numbers should be internally consistent within each report
