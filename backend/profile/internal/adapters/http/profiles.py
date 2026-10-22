import logging
import uuid
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, status

from internal.adapters.http.dependencies import get_current_user_id, get_profile_service
from internal.adapters.http.schemas import (
    BalanceResponse,
    CreateBusinessProfileRequest,
    OrderResponse,
    ProfileResponse,
    ProfileUpdateRequest,
)
from internal.app.service import ProfileService
from internal.domain.exceptions import ProfileAlreadyExistsError, ProfileNotFoundError

router = APIRouter(prefix="/profiles", tags=["Profiles"])

UserIdDep = Annotated[uuid.UUID, Depends(get_current_user_id)]
ProfileServiceDep = Annotated[ProfileService, Depends(get_profile_service)]

logger = logging.getLogger(__name__)


@router.get("/me", response_model=list[ProfileResponse])
async def get_my_profiles(user_id: UserIdDep, profile_service: ProfileServiceDep):
    """
    Получить все профили (CUSTOMER и/или BUSINESS) текущего пользователя.
    user_id берется безопасно из JWT токена.
    """
    profiles = await profile_service.get_profiles_by_user(user_id)
    return profiles


@router.patch("/me/{profile_id}", response_model=ProfileResponse)
async def update_my_profile(
    profile_id: uuid.UUID,
    update_data: ProfileUpdateRequest,
    user_id: UserIdDep,
    profile_service: ProfileServiceDep,
):
    """
    Обновить данные конкретного профиля.
    """
    try:
        updated_profile = await profile_service.update_profile(
            profile_id=profile_id,
            user_id=user_id,
            update_data=update_data.model_dump(exclude_unset=True),
        )
        return updated_profile

    except ProfileNotFoundError:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Profile not found or access denied",
        ) from None
    except Exception:  # noqa: BLE001
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal server error",
        ) from None


@router.get("/orders", response_model=list[OrderResponse])
async def get_my_orders(user_id: UserIdDep, profile_service: ProfileServiceDep):
    """Получить историю заказов текущего пользователя."""
    orders = await profile_service.get_orders(user_id)
    return orders


@router.get("/balance", response_model=BalanceResponse)
async def get_my_balance(user_id: UserIdDep, profile_service: ProfileServiceDep):
    """Получить баланс текущего пользователя."""
    balance = await profile_service.get_balance(user_id)
    return balance


@router.post(
    "/business",
    response_model=ProfileResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Создать бизнес-профиль",
    description="Создает профиль продавца (BUSINESS) для текущего пользователя.",
)
async def create_business_profile_endpoint(
    request_data: CreateBusinessProfileRequest,
    user_id: Annotated[uuid.UUID, Depends(get_current_user_id)],
    profile_service: Annotated[ProfileService, Depends(get_profile_service)],
):
    try:
        created_profile = await profile_service.create_business_profile(
            user_id=user_id, data=request_data
        )
        return created_profile

    except ProfileAlreadyExistsError as e:
        logger.warning(f"Conflict: business profile for user {user_id} already exists.")
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT, detail=str(e)
        ) from None

    except Exception:
        logger.exception(
            f"Unexpected error while creating business profile for user {user_id}"
        )
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal Server Error",
        ) from None
