document.addEventListener("DOMContentLoaded", () => {
   const slideshowTime = 5000;

   if (fsLightbox) {
      fsLightbox.props.slideshowTime = slideshowTime;
      fsLightbox.props.disableBackgroundClose = true;
   }

   htmx.on("htmx:afterSettle", () => {
      refreshFsLightbox();

      if (fsLightbox) {
         fsLightbox.props.slideshowTime = slideshowTime;
         fsLightbox.props.disableBackgroundClose = true;
      }
   });
});
