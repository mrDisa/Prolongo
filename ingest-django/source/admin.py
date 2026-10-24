from django.contrib import admin

from .models import Account, Service, StatusEvent, Transaction

admin.site.site_header = "Prolongo — данные источника"
admin.site.site_title = "Prolongo"
admin.site.index_title = "Аккаунты, услуги, транзакции и история статусов"


def format_eur(cents):
    """12345 -> "€123,45" (integer cents only, no floats)."""
    sign = "−" if cents < 0 else ""
    euros, rest = divmod(abs(cents), 100)
    return f"{sign}€{euros:,}".replace(",", " ") + f",{rest:02d}"


def eur_column(field, title):
    def column(obj):
        return format_eur(getattr(obj, field))

    column.short_description = title
    column.admin_order_field = field
    return column


@admin.register(Account)
class AccountAdmin(admin.ModelAdmin):
    list_display = (
        "id",
        "email",
        "balance",
        eur_column("balance", "Баланс, €"),
        "total_replenished",
        eur_column("total_replenished", "Пополнено, €"),
        "bonus_balance",
        "bonus_state",
    )
    list_filter = ("bonus_state",)
    search_fields = ("email",)
    ordering = ("id",)


@admin.register(Service)
class ServiceAdmin(admin.ModelAdmin):
    list_display = (
        "id",
        "name",
        "owner",
        "status",
        "payment_term",
        "auto_prolong",
        "expires_at",
        "price",
        eur_column("price", "Цена, €"),
    )
    list_editable = ("status", "auto_prolong")
    list_filter = ("status", "payment_term", "auto_prolong")
    list_select_related = ("owner",)
    search_fields = ("name", "owner__email")
    autocomplete_fields = ("owner",)
    ordering = ("id",)


@admin.register(Transaction)
class TransactionAdmin(admin.ModelAdmin):
    list_display = ("id", "service", "type", "status", "amount", "created_at", "performed_at")
    list_filter = ("type", "status")
    list_select_related = ("service",)
    search_fields = ("service__name",)
    autocomplete_fields = ("service",)
    ordering = ("-id",)


@admin.register(StatusEvent)
class StatusEventAdmin(admin.ModelAdmin):
    """Read-only: the history is written by PostgreSQL triggers."""

    list_display = ("id", "service_id", "account_id", "from_status", "to_status", "occurred_at", "source", "batch_id")
    list_filter = ("source", "to_status")
    search_fields = ("=service_id", "=account_id")
    ordering = ("-occurred_at", "-id")

    def has_add_permission(self, request):
        return False

    def has_change_permission(self, request, obj=None):
        return False

    def has_delete_permission(self, request, obj=None):
        return False
