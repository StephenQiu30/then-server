window.addEventListener("load", () => {
  SwaggerUIBundle({
    url: "/openapi.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
    displayOperationId: true,
    displayRequestDuration: true,
    docExpansion: "list",
    filter: true,
    validatorUrl: null,
    queryConfigEnabled: false,
    persistAuthorization: false,
    supportedSubmitMethods: ["get", "post", "put", "delete", "patch"],
    tryItOutEnabled: true,
    withCredentials: true,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
});
