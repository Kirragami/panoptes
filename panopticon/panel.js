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

	document.body.addEventListener("panel:access-granted", () => {
		const status = document.getElementById("login-auth");
		const target = status?.dataset.redirect || "/panel/";

		window.setTimeout(() => {
			window.location.assign(target);
		}, 650);
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
