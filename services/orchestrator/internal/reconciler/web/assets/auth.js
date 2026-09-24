const form = document.querySelector("[data-auth-form]");
const message = document.querySelector("[data-auth-message]");

form?.addEventListener("submit", async (event) => {
  event.preventDefault();
  message.textContent = "Connexion au control plane...";
  const submit = form.querySelector("button[type=submit]");
  submit.disabled = true;
  const payload = Object.fromEntries(new FormData(form).entries());
  try {
    const response = await fetch(form.dataset.endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "Une erreur est survenue");
    window.location.href = data.role === "staff" || data.role === "admin" ? "/staff" : "/dashboard";
  } catch (error) {
    message.textContent = error.message;
    message.dataset.state = "error";
    submit.disabled = false;
  }
});
