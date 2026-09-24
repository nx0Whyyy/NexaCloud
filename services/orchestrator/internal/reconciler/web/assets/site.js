document.querySelectorAll("[data-year]").forEach((node) => { node.textContent = new Date().getFullYear(); });

const observer = new IntersectionObserver((entries) => {
  entries.forEach((entry) => {
    if (entry.isIntersecting) {
      entry.target.classList.add("is-visible");
      observer.unobserve(entry.target);
    }
  });
}, { threshold: 0.14 });

document.querySelectorAll(".reveal").forEach((node) => observer.observe(node));

document.querySelectorAll("[data-logout]").forEach((button) => {
  button.addEventListener("click", async () => {
    await fetch("/api/v1/auth/logout", { method: "POST" });
    window.location.href = "/";
  });
});
