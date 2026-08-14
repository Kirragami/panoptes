(() => {
	"use strict";

	const localizeTimes = () => {
		const formatter = new Intl.DateTimeFormat(undefined, {
			dateStyle: "medium",
			timeStyle: "medium",
		});

		for (const element of document.querySelectorAll(
			"time.local-time[data-unix]",
		)) {
			const unix = Number(element.dataset.unix);
			if (!Number.isFinite(unix) || unix < 1) {
				continue;
			}

			const date = new Date(unix * 1000);
			element.dateTime = date.toISOString();
			element.title = date.toString();
			element.textContent = formatter.format(date);
		}
	};

	const dismissAccessDenied = () => {
		const status = document.querySelector("#login-auth .access-denied");
		if (!status || status.dataset.dismissScheduled === "true") {
			return;
		}

		status.dataset.dismissScheduled = "true";
		window.setTimeout(() => {
			status.classList.add("is-dismissing");
			window.setTimeout(() => {
				status.remove();
				document
					.getElementById("login-password")
					?.classList.remove("login-password-error");
			}, 220);
		}, 3000);
	};

	const copyText = async (value) => {
		if (navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(value);
			return;
		}

		const input = document.createElement("textarea");
		input.value = value;
		input.setAttribute("readonly", "");
		input.style.position = "fixed";
		input.style.opacity = "0";
		document.body.append(input);
		input.select();
		const copied = document.execCommand("copy");
		input.remove();
		if (!copied) {
			throw new Error("copy failed");
		}
	};

	const openOracleSigilViewer = (trigger) => {
		const source = trigger.querySelector("img");
		const viewer = document.getElementById("oracle-sigil-viewer");
		const image = document.getElementById("oracle-sigil-viewer-image");
		if (
			!(source instanceof HTMLImageElement) ||
			!(viewer instanceof HTMLDialogElement) ||
			!(image instanceof HTMLImageElement)
		) {
			return;
		}

		image.src = source.src;
		if (!viewer.open) {
			viewer.showModal();
		}
	};

	const closeOracleSigilViewer = () => {
		const viewer = document.getElementById("oracle-sigil-viewer");
		if (viewer instanceof HTMLDialogElement && viewer.open) {
			viewer.close();
		}
	};

	const activeSeal = (kind) => {
		const slug = kind.toLowerCase();
		const card = document.querySelector(`.seal-card-${slug}`);
		const seal = card?.querySelector(
			`#${slug}-seal-result .seal-output[data-seal-kind="${kind}"]`,
		);
		if (!card || !seal) {
			return null;
		}

		const sealID = seal.dataset.sealId;
		const expiresAt = seal.dataset.sealExpiresAt;
		if (!sealID || !expiresAt) {
			return null;
		}

		if (card.dataset.activeSealId !== sealID) {
			card.dataset.activeSealId = sealID;
			card.classList.remove("is-consuming", "is-consumed");
		}

		return { card, expiresAt, sealID };
	};

	const playSealConsumption = (kind, sealID) => {
		const active = activeSeal(kind);
		if (
			!active ||
			active.sealID !== sealID ||
			active.card.classList.contains("is-consuming") ||
			active.card.classList.contains("is-consumed")
		) {
			return;
		}

		active.card.classList.add("is-consuming");
		window.setTimeout(() => {
			if (active.card.dataset.activeSealId !== sealID) {
				return;
			}
			active.card.classList.remove("is-consuming");
			active.card.classList.add("is-consumed");
		}, kind === "Oracle" ? 1700 : 1500);
	};

	const refreshSealHistory = () => {
		if (!document.getElementById("seal-history") || !window.htmx?.ajax) {
			return;
		}
		window.htmx.ajax("GET", "/panel/fragments/seal-history", {
			target: "#seal-history",
			swap: "outerHTML",
		});
	};

	const syncSealConsumption = (kind) => {
		const active = activeSeal(kind);
		if (!active) {
			return;
		}

		const consumed = [...document.querySelectorAll(
			"#seal-history tr[data-seal-kind]",
		)].some(
			(row) =>
				row.dataset.sealKind === kind &&
				row.dataset.sealExpiresAt === active.expiresAt &&
				row.dataset.sealAvailability === "Consumed",
		);
		if (consumed) {
			playSealConsumption(kind, active.sealID);
		}
	};

	const connectSealEvents = () => {
		if (!window.EventSource) {
			return;
		}

		const events = new EventSource("/panel/events/seals");
		events.addEventListener("seal-consumed", (event) => {
			try {
				const consumption = JSON.parse(event.data);
				if (
					(consumption.kind !== "Oracle" && consumption.kind !== "Eye") ||
					typeof consumption.seal_id !== "string"
				) {
					return;
				}
				playSealConsumption(consumption.kind, consumption.seal_id);
				refreshSealHistory();
			} catch {
				// Ignore malformed stream messages and await the next event.
			}
		});
	};

	document.body.addEventListener("panel:access-granted", () => {
		const status = document.getElementById("login-auth");
		const target = status?.dataset.redirect || "/panel/";

		window.setTimeout(() => {
			window.location.assign(target);
		}, 650);
	});

	document.body.addEventListener("click", async (event) => {
		if (!(event.target instanceof Element)) {
			return;
		}
		const sigilTrigger = event.target.closest("[data-open-oracle-sigil]");
		if (sigilTrigger) {
			openOracleSigilViewer(sigilTrigger);
			return;
		}

		const closeSigilViewer = event.target.closest(
			"[data-close-oracle-sigil]",
		);
		if (closeSigilViewer) {
			closeOracleSigilViewer();
			return;
		}

		const button = event.target.closest("[data-copy-seal]");
		if (!button) {
			return;
		}

		const seal = button
			.closest(".seal-value")
			?.querySelector("[data-seal-value]")
			?.textContent?.trim();
		if (!seal) {
			return;
		}

		try {
			await copyText(seal);
			button.classList.add("is-copied");
			button.setAttribute("aria-label", "Copied");
		} catch {
			button.classList.add("is-failed");
			button.setAttribute("aria-label", "Copy failed");
		}
		window.setTimeout(() => {
			button.classList.remove("is-copied", "is-failed");
			button.setAttribute("aria-label", "Copy Seal");
		}, 1400);
	});

	document.addEventListener("DOMContentLoaded", () => {
		localizeTimes();
		dismissAccessDenied();
		syncSealConsumption("Eye");
		syncSealConsumption("Oracle");
		connectSealEvents();
	});
	document.body.addEventListener("htmx:afterSwap", () => {
		localizeTimes();
		dismissAccessDenied();
		syncSealConsumption("Eye");
		syncSealConsumption("Oracle");
	});
})();
