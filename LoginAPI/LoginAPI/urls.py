from django.contrib import admin
from django.urls import path, include
from login_api import urls as login_urls

urlpatterns = [

    path('admin/', admin.site.urls),
    path('api-auth/', include('rest_framework.urls')),
    path('login/', include(login_urls)),
]
