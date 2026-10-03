import uuid
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, status
from internal.adapters.http.dependencies import get_current_user_id, get_profile_service
from internal.adapters.http.schemas import ProfileResponse, ProfileUpdateRequest
from internal.domain.exceptions import ProfileNotFoundError

router = APIRouter(prefix="/profiles", tags=["Profiles"])

# Выносим зависимости в алиасы для переиспользования (очень частая практика)
UserIdDep = Annotated[uuid.UUID, Depends(get_current_user_id)]
# ProfileServiceDep = Annotated[ProfileService, Depends(get_profile_service)]
# Временно оставим как Any, пока не напишем сам сервис
ProfileServiceDep = Annotated[any, Depends(get_profile_service)]


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
        )
    except Exception:  # noqa: BLE001
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal server error",
        )
