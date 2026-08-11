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
	});
	document.body.addEventListener("htmx:afterSwap", () => {
		localizeTimes();
		dismissAccessDenied();
	});
})();
