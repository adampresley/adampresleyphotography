document.addEventListener("DOMContentLoaded", () => {
   htmx.on("htmx:afterSettle", () => {
      refreshFsLightbox();
   });
});
