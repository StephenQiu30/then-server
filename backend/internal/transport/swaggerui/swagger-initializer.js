window.addEventListener("load", () => {
  SwaggerUIBundle({
    url: "/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    displayOperationId: true,
    validatorUrl: null,
    queryConfigEnabled: false,
    persistAuthorization: false,
    supportedSubmitMethods: [],
    tryItOutEnabled: false,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
});
