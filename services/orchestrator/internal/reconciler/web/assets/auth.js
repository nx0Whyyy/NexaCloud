const form = document.querySelector("[data-auth-form]");
const message = document.querySelector("[data-auth-message]");

document.querySelectorAll("[data-toggle-password]").forEach((button) => button.addEventListener("click", () => {
  const input = button.parentElement.querySelector("input");
  const visible = input.type === "text";
  input.type = visible ? "password" : "text";
  button.textContent = visible ? "Afficher" : "Masquer";
}));

const password = document.querySelector('[name="password"]');
const confirmation = document.querySelector('[name="password_confirmation"]');
const strength = document.querySelector("[data-password-strength]");
const strengthLabel = document.querySelector("[data-strength-label]");
const rules = {
  length: (value) => value.length >= 12,
  case: (value) => /[a-z]/.test(value) && /[A-Z]/.test(value),
  number: (value) => /\d/.test(value),
  symbol: (value) => /[^A-Za-z0-9\s]/.test(value),
};

function updateStrength() {
  if (!password || !strength) return;
  const states = Object.entries(rules).map(([name, check]) => [name, check(password.value)]);
  states.forEach(([name, valid]) => document.querySelector(`[data-rule="${name}"]`)?.classList.toggle("valid", valid));
  const score = states.filter(([, valid]) => valid).length;
  strength.value = score;
  strength.dataset.score = score;
  strengthLabel.textContent = ["Très faible", "Faible", "Moyen", "Bon", "Robuste"][score];
  if (confirmation?.value) confirmation.setCustomValidity(confirmation.value === password.value ? "" : "Les mots de passe ne correspondent pas.");
}

password?.addEventListener("input", updateStrength);
confirmation?.addEventListener("input", updateStrength);

form?.addEventListener("submit", async (event) => {
  event.preventDefault();
  updateStrength();
  if (!form.reportValidity()) return;
  message.textContent = form.dataset.register !== undefined ? "Création sécurisée du compte..." : "Connexion au control plane...";
  message.dataset.state = "";
  const submit = form.querySelector('button[type="submit"]');
  submit.disabled = true;
  const payload = Object.fromEntries(new FormData(form).entries());
  delete payload.terms;
  try {
    const response = await fetch(form.dataset.endpoint, { method: "POST", headers: { "Content-Type": "application/json", Accept: "application/json" }, body: JSON.stringify(payload) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "Une erreur est survenue");
    if (data.verification_required) {
	  form.querySelectorAll("label, .password-strength, button").forEach((element) => { element.hidden = true; });
      message.textContent = `Un lien de vérification a été envoyé à ${data.email}.`;
      message.dataset.state = "success";
      return;
    }
    window.location.href = ["support", "moderator", "admin", "owner", "staff"].includes(data.role) ? "/staff" : "/dashboard";
  } catch (error) {
    message.textContent = error.message;
    message.dataset.state = "error";
    submit.disabled = false;
    if (error.message.includes("Vérifiez votre adresse")) {
      const resend = document.querySelector("[data-resend-form]");
      resend.hidden = false;
      resend.elements.email.value = form.elements.email.value;
    }
  }
});

document.querySelector("[data-resend-form]")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = event.currentTarget.querySelector("button");
  button.disabled = true;
  try {
    await fetch("/api/v1/auth/resend-verification", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: event.currentTarget.elements.email.value }) });
    button.textContent = "E-mail envoyé";
  } catch { button.textContent = "Envoi indisponible"; }
});

async function verifyEmail() {
  const target = document.querySelector("[data-verification]");
  if (!target) return;
  const token = new URLSearchParams(window.location.hash.slice(1)).get("token");
  history.replaceState(null, "", "/verify");
  const status = document.querySelector("[data-verify-message]");
  try {
    const response = await fetch("/api/v1/auth/verify", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token }) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error);
    target.querySelector("h1").textContent = "Adresse vérifiée.";
    status.textContent = "Votre compte NexaCloud est actif. Vous pouvez maintenant vous connecter.";
    document.querySelector("[data-verify-login]").hidden = false;
  } catch (error) {
    target.querySelector("h1").textContent = "Lien invalide.";
    status.textContent = error.message || "Ce lien est invalide ou a expiré.";
  }
}

verifyEmail();
