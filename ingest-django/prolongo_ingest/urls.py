from django.contrib import admin
from django.http import JsonResponse
from django.urls import include, path


def healthz(request):
    # No auth: used by the compose healthcheck; nginx does not proxy it.
    return JsonResponse({"status": "ok"})


urlpatterns = [
    path("healthz", healthz),
    path("admin/", admin.site.urls),
    path("api/v1/source/", include("source.urls")),
]
