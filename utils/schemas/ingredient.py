"Module for ingredient schema definitions using Pydantic."
# Built-in python modules
from typing import Dict, List

# Third-party modules
from pydantic import BaseModel


class ingredient(BaseModel):
    name: str
    display_name: str
    unit: Dict[str, int]
    food_group: str
    allergens: List[str]
