package main

const panelTemplateSource = `
{{define "head"}}
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta name="robots" content="noindex, nofollow">
	<title>Panopticon</title>
	<link rel="stylesheet" href="/panel/static/panel.css">
	<script src="/panel/static/htmx.min.js" defer></script>
	<script src="/panel/static/panel.js" defer></script>
</head>
{{end}}

{{define "local-time"}}
{{if panelUnix .}}<time class="local-time" data-unix="{{panelUnix .}}">{{formatPanelTime .}}</time>{{else}}never{{end}}
{{end}}

{{define "login-auth"}}
<form
	id="login-auth"
	action="/panel/login"
	method="post"
	class="login-form"
	hx-post="/panel/login"
	hx-target="#login-auth"
	hx-swap="outerHTML"
>
	<input type="hidden" name="next" value="{{.Next}}">
	<input
		id="login-password"
		class="login-password {{if .Error}}login-password-error{{end}}"
		type="password"
		name="password"
		autocomplete="current-password"
		autofocus
		required
		aria-label="Password"
	>
	{{if .Error}}<p class="login-access-status access-denied" role="alert">{{.Error}}</p>{{end}}
	<button type="submit" class="sr-only">Sign in</button>
</form>
{{end}}

{{define "login-granted"}}
<section
	id="login-auth"
	class="login-granted login-access-status access-granted"
	data-redirect="{{.Next}}"
	role="status"
>
	ACCESS GRANTED
</section>
{{end}}

{{define "login"}}
<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta name="robots" content="noindex, nofollow">
	<title>Panopticon</title>
	<link rel="stylesheet" href="/panel/static/panel.css">
	<script src="/panel/static/htmx.min.js" defer></script>
	<script src="/panel/static/panel.js" defer></script>
</head>
<body class="login-page">
	<main class="login-shell">
		<h1 class="login-title">PANOPTICON</h1>
		<figure class="login-decoration" aria-hidden="true">
			<img src="/panel/static/login-decoration.png" alt="">
		</figure>
		{{template "login-auth" .}}
	</main>
</body>
</html>
{{end}}

{{define "sidebar"}}
<aside class="sidebar">
	<div class="brand">Panopticon</div>
	<nav class="side-nav" aria-label="Panopticon navigation">
		<form action="/panel/" method="get">
			<button
				type="submit"
				class="nav-button {{if eq .CurrentTab "eyes"}}is-active{{end}}"
			>
				Eyes
			</button>
		</form>
		<form action="/panel/seals" method="get">
			<button
				type="submit"
				class="nav-button {{if eq .CurrentTab "seals"}}is-active{{end}}"
			>
				Seals
			</button>
		</form>
		<form action="/panel/oracles" method="get">
			<button
				type="submit"
				class="nav-button {{if eq .CurrentTab "oracles"}}is-active{{end}}"
			>
				Oracles
			</button>
		</form>
		<form action="/panel/omens" method="get">
			<button
				type="submit"
				class="nav-button {{if eq .CurrentTab "omens"}}is-active{{end}}"
			>
				Omens
			</button>
		</form>
	</nav>
	<form action="/panel/logout" method="post" class="sign-out">
		<input type="hidden" name="csrf" value="{{.CSRF}}">
		<button type="submit" class="nav-button">Sign out</button>
	</form>
</aside>
{{end}}

{{define "eyes"}}
<!doctype html>
<html lang="en">
{{template "head" .}}
<body>
	<div class="app-shell">
		{{template "sidebar" .}}
		<main class="workspace">
			<header class="page-header">
				<div>
					<h1>Eyes</h1>
				</div>
				<form action="/panel/" method="get" class="search-form">
					<input
						type="search"
						name="q"
						value="{{.Query}}"
						placeholder="Filter by Eye ID"
						aria-label="Filter Eyes by ID"
					>
				<button type="submit" class="button-wide">Filter</button>
				</form>
			</header>

			<section class="data-section">
				<div class="eye-summary">
					<span>All Eyes <strong>{{.Summary.All}}</strong></span>
					<span>Open Eyes <strong>{{.Summary.Open}}</strong></span>
					<span>Closed Eyes <strong>{{.Summary.Closed}}</strong></span>
					<span>Sigils Watched <strong>{{.Summary.SigilCount}}</strong></span>
				</div>
				<table class="data-table">
					<thead>
						<tr>
							<th aria-label="Status"></th>
							<th>Eye ID</th>
							<th>Sigils</th>
							<th>First seen</th>
							<th>Last seen</th>
						</tr>
					</thead>
					<tbody
						id="eyes"
						hx-get="/panel/fragments/eyes?page={{.Pagination.Page}}{{if .Query}}&amp;q={{.Query}}{{end}}"
						hx-trigger="every 5s"
						hx-swap="innerHTML"
					>
						{{template "eye-rows" .}}
					</tbody>
				</table>
				<footer class="list-footer">
					<span>{{.Pagination.Total}} Eyes · page {{.Pagination.Page}} of {{.Pagination.TotalPages}}</span>
					<div class="pagination">
						{{if .Pagination.HasPrevious}}
						<form action="/panel/" method="get">
							<input type="hidden" name="page" value="{{.Pagination.PreviousPage}}">
							{{if .Query}}<input type="hidden" name="q" value="{{.Query}}">{{end}}
							<button type="submit" class="button-secondary">Previous</button>
						</form>
						{{end}}
						{{if .Pagination.HasNext}}
						<form action="/panel/" method="get">
							<input type="hidden" name="page" value="{{.Pagination.NextPage}}">
							{{if .Query}}<input type="hidden" name="q" value="{{.Query}}">{{end}}
							<button type="submit" class="button-secondary">Next</button>
						</form>
						{{end}}
					</div>
				</footer>
			</section>
		</main>
	</div>
</body>
</html>
{{end}}

{{define "eye-rows"}}
{{if .Eyes}}
	{{range .Eyes}}
	<tr>
		<td>
			{{if .Online}}
			<span class="eye-state eye-open" title="Online">
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
					<path d="M2.5 12s3.4-6 9.5-6 9.5 6 9.5 6-3.4 6-9.5 6-9.5-6-9.5-6Z"/>
					<circle cx="12" cy="12" r="2.5"/>
				</svg>
				<span class="sr-only">Online</span>
			</span>
			{{else}}
			<span class="eye-state eye-closed" title="Offline">
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
					<path d="M2.5 12s3.4-6 9.5-6c1.7 0 3.1.5 4.3 1.2M21.5 12s-3.4 6-9.5 6c-1.7 0-3.1-.5-4.3-1.2"/>
					<path d="m4 4 16 16"/>
				</svg>
				<span class="sr-only">Offline</span>
			</span>
			{{end}}
		</td>
		<td>
			<form action="/panel/eyes/{{.ID}}" method="get">
				<button type="submit" class="row-button">{{.ID}}</button>
			</form>
		</td>
		<td>
			{{if .Sigils}}
				<div class="sigil-list">
					{{range .Sigils}}<span class="sigil">{{.}}</span>{{end}}
				</div>
			{{else}}
				<span class="quiet">—</span>
			{{end}}
		</td>
		<td>{{template "local-time" .FirstSeen}}</td>
		<td>{{template "local-time" .LastSeen}}</td>
	</tr>
	{{end}}
{{else}}
	<tr>
		<td colspan="5" class="empty-row">No Eyes match this view.</td>
	</tr>
{{end}}
{{end}}

{{define "eye-detail"}}
<!doctype html>
<html lang="en">
{{template "head" .}}
<body>
	<div class="app-shell">
		{{template "sidebar" .}}
		<main class="workspace">
			<header class="page-header page-header-detail">
				<div>
					<p class="page-kicker">Eye</p>
					<h1>{{.Eye.ID}}</h1>
					<p>First seen {{template "local-time" .Eye.FirstSeen}} · last seen {{template "local-time" .Eye.LastSeen}}</p>
				</div>
				<div class="header-actions">
					{{template "eye-status" .Eye}}
					<form action="/panel/" method="get">
						<button type="submit" class="button-secondary">All Eyes</button>
					</form>
				</div>
			</header>

			<section class="data-section">
				<div class="section-title">
					<h2>Reported Visions</h2>
				</div>
				{{if .Visions}}
				<table class="data-table">
					<thead>
						<tr>
							<th>Vision</th>
							<th>Form</th>
							<th>State</th>
							<th>Reported</th>
						</tr>
					</thead>
					<tbody>
						{{range .Visions}}
						<tr>
							<td>{{.Sight}}</td>
							<td>{{.Form}}</td>
							<td>
								{{if .Awake}}
									<span class="status status-good">Available</span>
								{{else}}
									<span class="status status-neutral">Unavailable</span>
									{{if .SlumberReason}}<span class="reason">{{.SlumberReason}}</span>{{end}}
								{{end}}
							</td>
							<td>{{template "local-time" .BeheldAt}}</td>
						</tr>
						{{end}}
					</tbody>
				</table>
				{{else}}
					<p class="empty-row">This Eye has not reported any Vision capabilities.</p>
				{{end}}
			</section>

			{{template "gaze-panel" .}}
		</main>
	</div>
</body>
</html>
{{end}}

{{define "eye-status"}}
<div
	id="eye-status"
	class="eye-status"
	hx-get="/panel/eyes/{{.ID}}/status"
	hx-trigger="every 5s"
	hx-swap="outerHTML"
>
	{{if .Online}}
		<span class="status status-good">Online</span>
	{{else}}
		<span class="status status-neutral">Offline</span>
	{{end}}
</div>
{{end}}

{{define "gaze-panel"}}
<section id="gaze-panel" class="data-section">
	<div class="section-title">
		<div>
			<h2>Monitoring</h2>
		</div>
	</div>
	{{if .Message}}<p class="notice" role="status">{{.Message}}</p>{{end}}
	{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}

	{{if .ConfigurableDockerHealth}}
	<form
		class="config-form"
		action="/panel/eyes/{{.Eye.ID}}/gazes"
		method="post"
		hx-post="/panel/eyes/{{.Eye.ID}}/gazes"
		hx-target="#gaze-panel"
		hx-swap="outerHTML"
	>
		<input type="hidden" name="csrf" value="{{.CSRF}}">
		<input type="hidden" name="vision" value="docker.health">
		<label>
			Target
			<input
				type="text"
				name="target"
				maxlength="512"
				placeholder="Docker service or container name"
				required
			>
		</label>
		<label>
			Sigil
			<input
				type="text"
				name="sigil"
				pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}"
				maxlength="64"
				placeholder="docker-health"
			>
		</label>
		<label>
			Interval
			<input
				type="number"
				name="reconcile_interval_seconds"
				min="1"
				max="86400"
				placeholder="60 seconds"
			>
		</label>
		<label>
			Grace
			<input
				type="number"
				name="starting_grace_seconds"
				min="0"
				max="86400"
				placeholder="120 seconds"
			>
		</label>
		<label class="checkbox-label">
			<input type="checkbox" name="awake" value="true" checked>
			<span>Enabled</span>
		</label>
		<button type="submit">Save</button>
	</form>
	{{else}}
		<p class="empty-row">
			This Eye needs to report docker.health before it can be configured.
		</p>
	{{end}}

	<div class="section-title monitor-title">
		<h2>Configured Sigils</h2>
	</div>
	{{if .Gazes}}
	<table class="data-table">
		<thead>
			<tr>
				<th>Sigil</th>
				<th>Target</th>
				<th>Interval</th>
				<th>State</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{{range .Gazes}}
			<tr>
				<td>{{.Sigil}}</td>
				<td>{{if .Target}}{{.Target}}{{else}}<span class="quiet">—</span>{{end}}</td>
				<td>{{if .ReconcileIntervalSeconds}}{{.ReconcileIntervalSeconds}}s{{else}}<span class="quiet">—</span>{{end}}</td>
				<td>
					{{if .Awake}}
						<span class="status status-good">Active</span>
					{{else}}
						<span class="status status-neutral">Paused</span>
					{{end}}
				</td>
				<td class="table-action">
					<form
						action="/panel/eyes/{{$.Eye.ID}}/gazes/{{.Sigil}}/toggle"
						method="post"
						hx-post="/panel/eyes/{{$.Eye.ID}}/gazes/{{.Sigil}}/toggle"
						hx-target="#gaze-panel"
						hx-swap="outerHTML"
					>
						<input type="hidden" name="csrf" value="{{$.CSRF}}">
						{{if .Awake}}
							<button class="button-secondary" type="submit">Pause</button>
						{{else}}
							<button type="submit">Resume</button>
						{{end}}
					</form>
				</td>
			</tr>
			{{end}}
		</tbody>
	</table>
	{{else}}
		<p class="empty-row">No Sigils are configured for this Eye.</p>
	{{end}}
</section>
{{end}}

{{define "oracles"}}
<!doctype html>
<html lang="en">
{{template "head" .}}
<body>
	<div class="app-shell">
		{{template "sidebar" .}}
		<main class="workspace">
			<header class="page-header page-header-detail">
				<div>
					<h1>Oracles</h1>
				</div>
				<form action="/panel/seals" method="get">
					<button type="submit" class="button-wide">Seals</button>
				</form>
			</header>

			<section class="data-section">
				{{if .Oracles}}
				<table class="data-table">
					<thead>
						<tr>
							<th>Oracle ID</th>
							<th>Paired</th>
							<th>State</th>
						</tr>
					</thead>
					<tbody>
						{{range .Oracles}}
						<tr>
							<td>{{.ID}}</td>
							<td>{{template "local-time" .PairedAt}}</td>
							<td>
								{{if .RevokedAt}}
									<span class="status status-neutral">Revoked</span>
								{{else}}
									<span class="status status-good">Active</span>
								{{end}}
							</td>
						</tr>
						{{end}}
					</tbody>
				</table>
				{{else}}
					<p class="empty-row">No mobile Oracles are paired yet.</p>
				{{end}}
			</section>
		</main>
	</div>
</body>
</html>
{{end}}

{{define "omens"}}
<!doctype html>
<html lang="en">
{{template "head" .}}
<body>
	<div class="app-shell">
		{{template "sidebar" .}}
		<main class="workspace">
			<header class="page-header">
				<div>
					<h1>Omens</h1>
				</div>
			</header>

			<section class="data-section">
				{{if .Omens}}
				<table class="data-table">
					<thead>
						<tr>
							<th>Eye ID</th>
							<th>Sigil</th>
							<th>Turn</th>
							<th>Received</th>
						</tr>
					</thead>
					<tbody>
						{{range .Omens}}
						<tr>
							<td>
								<form action="/panel/eyes/{{.EyeID}}" method="get">
									<button type="submit" class="row-button">{{.EyeID}}</button>
								</form>
							</td>
							<td><span class="sigil">{{.GazeSigil}}</span></td>
							<td>{{.GazeTurn}}</td>
							<td>{{template "local-time" .ReceivedAt}}</td>
						</tr>
						{{end}}
					</tbody>
				</table>
				{{else}}
					<p class="empty-row">No Omens have been received yet.</p>
				{{end}}
			</section>
		</main>
	</div>
</body>
</html>
{{end}}

{{define "seals"}}
<!doctype html>
<html lang="en">
{{template "head" .}}
<body>
	<div class="app-shell">
		{{template "sidebar" .}}
		<main class="workspace">
			<header class="page-header">
				<div>
					<h1>Seals</h1>
				</div>
			</header>

			{{template "seals-content" .}}
		</main>
	</div>
</body>
</html>
{{end}}

{{define "seals-content"}}
<div id="seals-content">
	<section class="seals-section">
		<article class="seal-card seal-card-eye">
			<div class="seal-card-main">
				<svg class="seal-type-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
					<path d="M2.5 12s3.4-6 9.5-6 9.5 6 9.5 6-3.4 6-9.5 6-9.5-6-9.5-6Z"/>
					<circle cx="12" cy="12" r="2.5"/>
				</svg>
				<h2>Eye</h2>
				<form
					class="seal-form"
					action="/panel/seals/eye"
					method="post"
					hx-post="/panel/seals/eye"
					hx-target="#eye-seal-result"
					hx-swap="innerHTML"
				>
					<input type="hidden" name="csrf" value="{{.CSRF}}">
					<button type="submit" name="confirm" value="forge" class="button-wide">
						Forge
					</button>
				</form>
				<div id="eye-seal-result" class="seal-result" aria-live="polite">
					{{template "eye-seal-result" .Eye}}
				</div>
			</div>
		</article>

		<article class="seal-card seal-card-oracle">
			<div class="seal-card-main">
				<svg class="seal-type-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
					<rect x="5.5" y="2.5" width="13" height="19" rx="2"/>
					<path d="M9.5 5.5h5"/>
					<circle cx="12" cy="18.5" r=".75" fill="currentColor" stroke="none"/>
				</svg>
				<h2>Oracle</h2>
				<form
					class="seal-form"
					action="/panel/seals/oracle"
					method="post"
					hx-post="/panel/seals/oracle"
					hx-target="#oracle-seal-result"
					hx-swap="innerHTML"
				>
					<input type="hidden" name="csrf" value="{{.CSRF}}">
					<button type="submit" name="confirm" value="forge" class="button-wide">
						Forge
					</button>
				</form>
			</div>
			<div id="oracle-seal-result" class="seal-result" aria-live="polite">
				{{template "oracle-seal-result" .Oracle}}
			</div>
			<div id="oracle-seal-sigil-slot" class="oracle-seal-sigil-slot">
				{{template "oracle-seal-sigil" .Oracle}}
				{{template "seal-consumed-eye" .}}
			</div>
		</article>
	</section>

	<dialog id="oracle-sigil-viewer" class="oracle-sigil-viewer" aria-labelledby="oracle-sigil-viewer-title">
		<div class="oracle-sigil-viewer-header">
			<p id="oracle-sigil-viewer-title">Oracle Seal</p>
			<button type="button" class="oracle-sigil-viewer-close" data-close-oracle-sigil aria-label="Close fullscreen Oracle Seal">
				×
			</button>
		</div>
		<img id="oracle-sigil-viewer-image" alt="Fullscreen Oracle pairing sigil">
		<p class="oracle-sigil-viewer-hint">Display this sigil at full size for the Oracle to receive.</p>
	</dialog>

	{{template "seal-history" .}}
</div>
{{end}}

{{define "seal-history"}}
<section
	id="seal-history"
	class="data-section seal-history-section"
	hx-get="/panel/fragments/seal-history"
	hx-trigger="every 5s"
	hx-swap="outerHTML"
>
	{{template "seal-history-table" .}}
</section>
{{end}}

{{define "seal-history-oob"}}
<section
	id="seal-history"
	class="data-section seal-history-section"
	hx-get="/panel/fragments/seal-history"
	hx-trigger="every 5s"
	hx-swap="outerHTML"
	hx-swap-oob="true"
>
	{{template "seal-history-table" .}}
</section>
{{end}}

{{define "seal-history-table"}}
<table class="data-table">
	<thead>
		<tr>
			<th>Type</th>
			<th>Forged</th>
			<th>Expires</th>
			<th>Availability</th>
			<th>Consumed</th>
		</tr>
	</thead>
	<tbody>
		{{if .SealHistory}}
			{{range .SealHistory}}
			<tr
				data-seal-kind="{{.Kind}}"
				data-seal-expires-at="{{panelUnix .ExpiresAt}}"
				data-seal-availability="{{.Availability}}"
			>
				<td>{{.Kind}}</td>
				<td>{{template "local-time" .ForgedAt}}</td>
				<td>{{template "local-time" .ExpiresAt}}</td>
				<td>
					{{if eq .Availability "Available"}}
						<span class="status status-seal-available">Available</span>
					{{else if eq .Availability "Consumed"}}
						<span class="status status-neutral">Consumed</span>
					{{else}}
						<span class="status status-seal-expired">Expired</span>
					{{end}}
				</td>
				<td>
					{{if .Consumed}}
						{{template "local-time" .ConsumedAt}}
					{{else}}
						<span class="quiet">—</span>
					{{end}}
				</td>
			</tr>
			{{end}}
		{{else}}
		<tr>
			<td colspan="5" class="empty-row">No Seals forged yet.</td>
		</tr>
		{{end}}
	</tbody>
</table>
{{end}}

{{define "eye-seal-outcome"}}
{{template "eye-seal-result" .Eye}}
{{template "seal-history-oob" .}}
{{end}}

{{define "oracle-seal-sigil"}}
{{if .SealImageDataURL}}
<button type="button" class="oracle-seal-sigil" data-open-oracle-sigil aria-label="Open fullscreen Oracle pairing sigil">
	<img src="{{.SealImageDataURL}}" alt="Oracle pairing sigil">
</button>
{{end}}
{{end}}

{{define "oracle-seal-sigil-oob"}}
<div id="oracle-seal-sigil-slot" class="oracle-seal-sigil-slot" hx-swap-oob="true">
	{{template "oracle-seal-sigil" .Oracle}}
	{{template "seal-consumed-eye" .}}
</div>
{{end}}

{{define "seal-consumed-eye"}}
<span class="seal-consumed-eye" aria-hidden="true">
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
		<path d="M2.5 12s3.4-6 9.5-6 9.5 6 9.5 6-3.4 6-9.5 6-9.5-6-9.5-6Z"/>
		<circle cx="12" cy="12" r="2.5"/>
	</svg>
</span>
{{end}}

{{define "oracle-seal-outcome"}}
{{template "oracle-seal-result" .Oracle}}
{{template "oracle-seal-sigil-oob" .}}
{{template "seal-history-oob" .}}
{{end}}

{{define "eye-seal-result"}}
{{if .Error}}
	<p class="error" role="alert">{{.Error}}</p>
{{end}}
{{if .Seal}}
	<div
		class="seal-output"
		data-seal-kind="Eye"
		data-seal-id="{{.SealID}}"
		data-seal-expires-at="{{panelUnix .ExpiresAt}}"
	>
		<div class="seal-value">
			<code class="seal-text" data-seal-value>{{.Seal}}</code>
			<button type="button" class="seal-copy" aria-label="Copy Seal" data-copy-seal>
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
					<rect x="8" y="8" width="11" height="11" rx="1"/>
					<path d="M16 8V5a1 1 0 0 0-1-1H5a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h3"/>
				</svg>
			</button>
		</div>
	</div>
{{end}}
{{end}}

{{define "oracle-seal-result"}}
{{if .Error}}
	<p class="error" role="alert">{{.Error}}</p>
{{end}}
{{if .Seal}}
	<div
		hidden
		class="seal-output"
		data-seal-kind="Oracle"
		data-seal-id="{{.SealID}}"
		data-seal-expires-at="{{panelUnix .ExpiresAt}}"
	></div>
{{end}}
{{end}}
`
