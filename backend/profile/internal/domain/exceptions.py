# internal/domain/exceptions.py


class ProfileNotFoundError(Exception):
    """Выбрасывается, когда профиль не найден в базе."""

    def __init__(self, profile_id: str):
        self.profile_id = profile_id
        super().__init__(f"Profile with ID {profile_id} not found.")


class ProfileAlreadyExistsError(Exception):
    """Выбрасывается, если нарушено бизнес-правило уникальности."""
