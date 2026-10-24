from django.urls import path

from . import views

urlpatterns = [
    path("snapshot", views.SnapshotView.as_view(), name="source-snapshot"),
    path("events", views.EventsView.as_view(), name="source-events"),
]
