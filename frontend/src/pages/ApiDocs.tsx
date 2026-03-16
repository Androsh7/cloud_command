import SwaggerUI from "swagger-ui-react";
import "swagger-ui-react/swagger-ui.css";

export default function ApiDocs() {
  return (
    <div className="container-fluid mt-3">
      <SwaggerUI url="/api/openapi.json" />
    </div>
  );
}
