"""路由聚合。"""
from fastapi import APIRouter

from app.routers import health
from app.routers import chat

router = APIRouter()
router.include_router(health.router)
router.include_router(chat.router, prefix="/api/v1/ai", tags=["ai"])
